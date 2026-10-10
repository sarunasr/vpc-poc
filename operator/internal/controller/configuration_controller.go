package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	configurationv1alpha1 "github.com/sarunasr/vpc-poc/operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

const (
	manualRequestAnnotation = "infra.example.io/reconcile-request"
	appLabel                = "app.kubernetes.io/managed-by"
	appLabelValue           = "infrastructure-gitops"
	configLabel             = "infra.example.io/configuration"
	hostLabel               = "infra.example.io/target"
	revisionLabel           = "infra.example.io/revision"
)

type ConfigurationReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	Image              string
	Reader             client.Reader
	LifecycleDir       string
	CollectionVersions map[string]string
}

func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configurationv1alpha1.Configuration{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var config configurationv1alpha1.Configuration
	if err := r.Get(ctx, req.NamespacedName, &config); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	revision, err := desiredRevision(config.Spec)
	if err != nil {
		return ctrl.Result{}, err
	}
	config.Status.DesiredRevision = revision
	if config.Spec.Infrastructure != nil {
		ready, err := r.workspaceReady(ctx, config.Namespace, config.Spec.Infrastructure.WorkspaceName)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !ready {
			setCondition(&config, "InfrastructureReady", metav1.ConditionFalse, "InfrastructureNotReady", "Referenced OpenTofu Workspace is not ready")
			setCondition(&config, "Ready", metav1.ConditionFalse, "InfrastructureNotReady", "Referenced OpenTofu Workspace is not ready")
			return r.updateStatus(ctx, &config, ctrl.Result{RequeueAfter: 30 * time.Second})
		}
	}
	setCondition(&config, "InfrastructureReady", metav1.ConditionTrue, "Ready", "Required infrastructure is ready")
	for _, collection := range config.Spec.Ansible.Collections {
		if r.CollectionVersions[collection.Name] != collection.Version {
			message := fmt.Sprintf("execution image contains %s:%s, but Configuration requests %s", collection.Name, r.CollectionVersions[collection.Name], collection.Version)
			setCondition(&config, "ConfigurationSynced", metav1.ConditionFalse, "CollectionVersionUnavailable", message)
			setCondition(&config, "Ready", metav1.ConditionFalse, "CollectionVersionUnavailable", message)
			return r.updateStatus(ctx, &config, ctrl.Result{RequeueAfter: 5 * time.Minute})
		}
	}
	if config.Status.AppliedRevision == revision && revision != "" {
		setCondition(&config, "ConfigurationSynced", metav1.ConditionTrue, "ConfigurationApplied", "Desired configuration revision is applied")
		setCondition(&config, "Ready", metav1.ConditionTrue, "ConfigurationApplied", "Infrastructure and configuration are ready")
	}

	manualRequest := config.Annotations[manualRequestAnnotation]
	due, nextWait, scheduledKey := scheduleDue(config, time.Now())
	runOnChange := config.Spec.Reconciliation.OnChange && config.Status.AppliedRevision != revision
	manual := manualRequest != "" && manualRequest != config.Status.LastManualRequest
	trigger := ""
	switch {
	case manual:
		trigger = "manual-" + manualRequest
	case runOnChange:
		trigger = "change-" + revision
	case due:
		trigger = "schedule-" + scheduledKey
	}
	if manual {
		config.Status.Attempt = 0
	}
	attemptKey := revision + "|" + trigger
	if trigger != "" && config.Status.AttemptedRevision != attemptKey {
		config.Status.Attempt = 0
		config.Status.AttemptedRevision = attemptKey
	}
	if trigger != "" && config.Status.Attempt >= maxRetries(config.Spec.Execution.MaxRetries) && !manual {
		setCondition(&config, "ConfigurationSynced", metav1.ConditionFalse, "RetryLimitReached", "Execution retry limit reached; change the desired configuration or request a manual run")
		return r.updateStatus(ctx, &config, ctrl.Result{})
	}

	if config.Status.ActiveJob != "" {
		var job batchv1.Job
		err := r.Get(ctx, types.NamespacedName{Namespace: config.Namespace, Name: config.Status.ActiveJob}, &job)
		if err == nil {
			switch {
			case jobSucceeded(&job):
				now := metav1.Now()
				config.Status.AppliedRevision = job.Labels[revisionLabel]
				config.Status.LastSuccessfulExecution = &now
				config.Status.LastVerificationTime = &now
				config.Status.LastCompletedExecution = &now
				config.Status.LastExecutionResult = "Succeeded"
				config.Status.LastExecutionError = ""
				config.Status.AppliedRoles = roleNames(config.Spec.Ansible.Roles)
				config.Status.AppliedCollections = append([]configurationv1alpha1.CollectionSpec(nil), config.Spec.Ansible.Collections...)
				config.Status.ActiveJob = ""
				config.Status.Attempt = 0
				setCondition(&config, "ConfigurationSynced", metav1.ConditionTrue, "ConfigurationApplied", "Ansible execution completed successfully")
				setCondition(&config, "Ready", metav1.ConditionTrue, "ConfigurationApplied", "Infrastructure and configuration are ready")
				if err := r.Status().Update(ctx, &config); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			case jobFailed(&job):
				now := metav1.Now()
				config.Status.ActiveJob = ""
				config.Status.LastCompletedExecution = &now
				config.Status.LastExecutionResult = "Failed"
				config.Status.LastExecutionError = jobFailure(&job)
				setCondition(&config, "ConfigurationSynced", metav1.ConditionFalse, "AnsibleExecutionFailed", config.Status.LastExecutionError)
				setCondition(&config, "Ready", metav1.ConditionFalse, "AnsibleExecutionFailed", config.Status.LastExecutionError)
				if config.Status.Attempt >= maxRetries(config.Spec.Execution.MaxRetries) {
					if err := r.Status().Update(ctx, &config); err != nil {
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				}
				if err := r.Delete(ctx, &job); err != nil && !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
				if err := r.Status().Update(ctx, &config); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: retryDelay(config.Status.Attempt)}, nil
			default:
				return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
			}
		} else if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		} else {
			config.Status.ActiveJob = ""
		}
	}

	if trigger == "" {
		if nextWait > 0 {
			return r.updateStatus(ctx, &config, ctrl.Result{RequeueAfter: nextWait})
		}
		return r.updateStatus(ctx, &config, ctrl.Result{})
	}

	available, err := r.checkConcurrency(ctx, &config)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !available {
		return ctrl.Result{RequeueAfter: 20 * time.Second}, nil
	}

	config.Status.Attempt++
	config.Status.AttemptedRevision = attemptKey
	jobName := executionName(config.Name, revision, trigger, config.Status.Attempt)
	job := buildJob(&config, jobName, revision, r.Image)
	if err := controllerutil.SetControllerReference(&config, job, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	project, inventory, cleanupWarnings, err := jobInputs(&config, r.LifecycleDir)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(cleanupWarnings) > 0 {
		setCondition(&config, "RoleCleanup", metav1.ConditionFalse, "CleanupUnsupported", strings.Join(cleanupWarnings, "; "))
	} else if config.Spec.Ansible.Lifecycle.UninstallRemovedRoles {
		setCondition(&config, "RoleCleanup", metav1.ConditionTrue, "CleanupComplete", "Removed roles with supported uninstall procedures were processed")
	}
	inputs := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: config.Namespace},
		Data:       map[string]string{"site.yml": project, "hosts": inventory},
	}
	if err := controllerutil.SetControllerReference(&config, inputs, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Create(ctx, inputs); err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, err
	}
	if err := r.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, err
	}
	now := metav1.Now()
	config.Status.ActiveJob = jobName
	config.Status.LastAttemptedExecution = &now
	if manual {
		config.Status.LastManualRequest = manualRequest
	}
	setCondition(&config, "ConfigurationSynced", metav1.ConditionUnknown, "ExecutionPending", "Ansible Job is running")
	if err := r.Status().Update(ctx, &config); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

func (r *ConfigurationReconciler) workspaceReady(ctx context.Context, namespace, name string) (bool, error) {
	obj := workspaceReadyObject(namespace, name)
	if err := r.Reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false, err
	}
	for _, item := range conditions {
		condition, ok := item.(map[string]interface{})
		if ok && condition["type"] == "Ready" && condition["status"] == "True" {
			return true, nil
		}
	}
	return false, nil
}

func (r *ConfigurationReconciler) checkConcurrency(ctx context.Context, config *configurationv1alpha1.Configuration) (bool, error) {
	var jobs batchv1.JobList
	if err := r.List(ctx, &jobs, client.InNamespace(config.Namespace), client.MatchingLabels{appLabel: appLabelValue}); err != nil {
		return false, err
	}
	active := int32(0)
	target := hostHash(config.Spec.Target.Host)
	for i := range jobs.Items {
		job := &jobs.Items[i]
		if jobFailed(job) || jobSucceeded(job) {
			continue
		}
		if job.Labels[hostLabel] == target {
			return false, nil
		}
		active++
	}
	max := config.Spec.Execution.MaxConcurrent
	if max <= 0 {
		max = 10
	}
	if active >= max {
		return false, nil
	}
	return true, nil
}

func (r *ConfigurationReconciler) updateStatus(ctx context.Context, config *configurationv1alpha1.Configuration, result ctrl.Result) (ctrl.Result, error) {
	var latest configurationv1alpha1.Configuration
	if err := r.Get(ctx, types.NamespacedName{Namespace: config.Namespace, Name: config.Name}, &latest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if equality.Semantic.DeepEqual(latest.Status, config.Status) {
		return result, nil
	}
	if err := r.Status().Update(ctx, config); err != nil {
		return ctrl.Result{}, err
	}
	return result, nil
}

func desiredRevision(spec configurationv1alpha1.ConfigurationSpec) (string, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("marshal desired configuration: %w", err)
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

func scheduleDue(config configurationv1alpha1.Configuration, now time.Time) (bool, time.Duration, string) {
	schedule := config.Spec.Reconciliation.Schedule
	if schedule == nil {
		return false, 0, ""
	}
	var last time.Time
	if config.Status.LastSuccessfulExecution != nil {
		last = config.Status.LastSuccessfulExecution.Time
	} else {
		last = config.CreationTimestamp.Time
	}
	if config.Status.LastAttemptedExecution != nil && config.Status.LastAttemptedExecution.After(last) {
		last = config.Status.LastAttemptedExecution.Time
	}
	switch schedule.Type {
	case "interval":
		duration, err := time.ParseDuration(schedule.Duration)
		if err != nil || duration <= 0 {
			return false, time.Minute, ""
		}
		next := last.Add(duration)
		if !now.Before(next) {
			return true, time.Second, next.UTC().Format(time.RFC3339)
		}
		return false, next.Sub(now), next.UTC().Format(time.RFC3339)
	case "cron":
		location, err := time.LoadLocation(schedule.Timezone)
		if err != nil {
			return false, time.Minute, ""
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		expression, err := parser.Parse(schedule.Expression)
		if err != nil {
			return false, time.Minute, ""
		}
		next := expression.Next(last.In(location))
		if !now.In(location).Before(next) {
			return true, time.Second, next.Format(time.RFC3339)
		}
		return false, next.Sub(now.In(location)), next.Format(time.RFC3339)
	default:
		return false, 0, ""
	}
}

func buildJob(config *configurationv1alpha1.Configuration, name, revision, image string) *batchv1.Job {
	targetName := config.Spec.Target.Host
	if len(targetName) > 63 {
		targetName = hostHash(targetName)
	}
	port := config.Spec.Target.Port
	if port <= 0 {
		port = 22
	}
	deadline := int64(1800)
	if config.Spec.Execution.Timeout.Duration > 0 {
		deadline = int64(config.Spec.Execution.Timeout.Duration.Seconds())
	}
	labels := map[string]string{appLabel: appLabelValue, configLabel: config.Name, hostLabel: hostHash(config.Spec.Target.Host), revisionLabel: revision[:12]}
	backoffLimit := int32(0)
	ttl := int32(86400)
	fsGroup := int64(1000)
	runAsUser := int64(1000)
	runAsNonRoot := true
	allowEscalation := false
	readOnlyRootFS := true
	mode := int32(0440)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: config.Namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &deadline,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyNever,
					AutomountServiceAccountToken: boolPointer(false),
					SecurityContext:              &corev1.PodSecurityContext{RunAsNonRoot: &runAsNonRoot, RunAsUser: &runAsUser, FSGroup: &fsGroup, SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}},
					Containers: []corev1.Container{{
						Name:            "ansible-runner",
						Image:           image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"ansible-runner", "run", "/runner", "-p", "site.yml"},
						Env: []corev1.EnvVar{
							{Name: "ANSIBLE_HOST_KEY_CHECKING", Value: "False"},
							{Name: "ANSIBLE_INVENTORY", Value: "/runner/inventory/hosts"},
							{Name: "ANSIBLE_PRIVATE_KEY_FILE", Value: "/credentials/ssh-privatekey"},
							{Name: "HOME", Value: "/tmp"},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "runner-data", MountPath: "/runner"},
							{Name: "project", MountPath: "/runner/project", ReadOnly: true},
							{Name: "inventory", MountPath: "/runner/inventory", ReadOnly: true},
							{Name: "ssh-key", MountPath: "/credentials", ReadOnly: true},
							{Name: "tmp", MountPath: "/tmp"},
						},
						SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: &allowEscalation, ReadOnlyRootFilesystem: &readOnlyRootFS, Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}},
					}},
					Volumes: []corev1.Volume{
						{Name: "runner-data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "project", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: name}, Items: []corev1.KeyToPath{{Key: "site.yml", Path: "site.yml"}}, DefaultMode: &mode}}},
						{Name: "inventory", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: name}, Items: []corev1.KeyToPath{{Key: "hosts", Path: "hosts"}}, DefaultMode: &mode}}},
						{Name: "ssh-key", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: config.Spec.Target.SSHSecretRef, DefaultMode: &mode, Items: []corev1.KeyToPath{{Key: "ssh-privatekey", Path: "ssh-privatekey", Mode: &mode}}}}},
						{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
					},
				},
			},
		},
	}
}

func jobInputs(config *configurationv1alpha1.Configuration, lifecycleDir string) (string, string, []string, error) {
	roles := append([]configurationv1alpha1.RoleSpec(nil), config.Spec.Ansible.Roles...)
	sort.Slice(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })
	roleEntries := make([]map[string]interface{}, 0, len(roles))
	desiredRoles := make(map[string]bool, len(roles))
	for _, role := range roles {
		desiredRoles[role.Name] = true
		entry := map[string]interface{}{"role": role.Name}
		if len(role.Vars) > 0 {
			entry["vars"] = role.Vars
		}
		roleEntries = append(roleEntries, entry)
	}
	var cleanupWarnings []string
	if config.Spec.Ansible.Lifecycle.UninstallRemovedRoles {
		for _, removedRole := range config.Status.AppliedRoles {
			if desiredRoles[removedRole] {
				continue
			}
			metadata, err := readLifecycleMetadata(lifecycleDir, removedRole)
			if err != nil {
				return "", "", nil, err
			}
			if !metadata.Lifecycle.Uninstall.Supported {
				cleanupWarnings = append(cleanupWarnings, removedRole+": uninstall is not supported; cleanup skipped")
				continue
			}
			if !roleVariable.MatchString(metadata.Lifecycle.Uninstall.Variable) || metadata.Lifecycle.Uninstall.Value == "" {
				return "", "", nil, fmt.Errorf("role %s has invalid uninstall metadata", removedRole)
			}
			roleCollection := strings.Split(removedRole, ".")[1]
			if !appliedCollectionMatches(config.Status.AppliedCollections, config.Spec.Ansible.Collections, roleCollection) {
				cleanupWarnings = append(cleanupWarnings, removedRole+": previous collection version is unavailable in the current execution image; cleanup skipped")
				continue
			}
			roleEntries = append(roleEntries, map[string]interface{}{
				"role": removedRole,
				"vars": map[string]interface{}{metadata.Lifecycle.Uninstall.Variable: metadata.Lifecycle.Uninstall.Value},
			})
		}
	}
	playbook, err := json.Marshal([]map[string]interface{}{{
		"name":   "Apply declared configuration",
		"hosts":  "all",
		"become": true,
		"roles":  roleEntries,
	}})
	if err != nil {
		return "", "", nil, fmt.Errorf("encode Ansible playbook: %w", err)
	}
	targetName := hostHash(config.Spec.Target.Host)
	port := config.Spec.Target.Port
	if port <= 0 {
		port = 22
	}
	inventory, err := json.Marshal(map[string]interface{}{
		"all": map[string]interface{}{"hosts": map[string]interface{}{
			targetName: map[string]interface{}{
				"ansible_host":                 config.Spec.Target.Host,
				"ansible_user":                 config.Spec.Target.User,
				"ansible_port":                 port,
				"ansible_ssh_private_key_file": "/credentials/ssh-privatekey",
				"ansible_ssh_common_args":      "-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10",
			},
		}},
	})
	if err != nil {
		return "", "", nil, fmt.Errorf("encode Ansible inventory: %w", err)
	}
	sort.Strings(cleanupWarnings)
	return string(playbook), string(inventory), cleanupWarnings, nil
}

type lifecycleMetadata struct {
	Lifecycle struct {
		Uninstall struct {
			Supported bool        `json:"supported"`
			Variable  string      `json:"variable"`
			Value     interface{} `json:"value"`
		} `json:"uninstall"`
	} `json:"lifecycle"`
}

var roleVariable = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func readLifecycleMetadata(baseDir, role string) (lifecycleMetadata, error) {
	parts := strings.Split(role, ".")
	if len(parts) != 3 {
		return lifecycleMetadata{}, fmt.Errorf("role name %q must have namespace.collection.role format", role)
	}
	if baseDir == "" {
		baseDir = "/usr/share/infrastructure-gitops/ansible_collections"
		if _, err := os.Stat(baseDir); err != nil {
			baseDir = filepath.Join("..", "..", "..", "ansible", "collections", "ansible_collections")
		}
	}
	path := filepath.Join(baseDir, parts[0], parts[1], "roles", parts[2], "meta", "gitops.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lifecycleMetadata{}, nil
		}
		return lifecycleMetadata{}, fmt.Errorf("read lifecycle metadata for %s: %w", role, err)
	}
	var metadata lifecycleMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return lifecycleMetadata{}, fmt.Errorf("parse lifecycle metadata for %s: %w", role, err)
	}
	return metadata, nil
}

func appliedCollectionMatches(applied, desired []configurationv1alpha1.CollectionSpec, collection string) bool {
	var appliedVersion, desiredVersion string
	for _, item := range applied {
		if strings.HasSuffix(item.Name, "."+collection) {
			appliedVersion = item.Version
		}
	}
	for _, item := range desired {
		if strings.HasSuffix(item.Name, "."+collection) {
			desiredVersion = item.Version
		}
	}
	return appliedVersion != "" && appliedVersion == desiredVersion
}

func executionName(configName, revision, trigger string, attempt int32) string {
	triggerHash := sha256.Sum256([]byte(trigger))
	if len(configName) > 20 {
		configName = strings.Trim(configName[:20], "-")
	}
	return fmt.Sprintf("%s-%.8s-%x-%d", configName, revision, triggerHash[:3], attempt)
}

func setCondition(config *configurationv1alpha1.Configuration, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{Type: conditionType, Status: status, Reason: reason, Message: message, ObservedGeneration: config.Generation}
	for i := range config.Status.Conditions {
		if config.Status.Conditions[i].Type == conditionType {
			condition.LastTransitionTime = config.Status.Conditions[i].LastTransitionTime
			if config.Status.Conditions[i].Status != status {
				condition.LastTransitionTime = metav1.Now()
			}
			config.Status.Conditions[i] = condition
			return
		}
	}
	condition.LastTransitionTime = metav1.Now()
	config.Status.Conditions = append(config.Status.Conditions, condition)
}

func jobSucceeded(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return job.Status.Succeeded > 0
}

func jobFailed(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func jobFailure(job *batchv1.Job) string {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed {
			return condition.Message
		}
	}
	return "Ansible Job failed"
}

func roleNames(roles []configurationv1alpha1.RoleSpec) []string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	sort.Strings(names)
	return names
}

func maxRetries(retries int32) int32 {
	if retries <= 0 {
		return 3
	}
	return retries
}

func retryDelay(attempt int32) time.Duration {
	delay := time.Duration(attempt) * 30 * time.Second
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func maxDuration(a, b time.Duration) time.Duration {
	if a < b {
		return b
	}
	return a
}

func hostHash(host string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(host)))
	return hex.EncodeToString(sum[:8])
}

func boolPointer(value bool) *bool {
	return &value
}

func workspaceReadyObject(namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "opentofu.m.upbound.io/v1beta1",
		"kind":       "Workspace",
		"metadata": map[string]interface{}{
			"namespace": namespace,
			"name":      name,
		},
	}}
}
