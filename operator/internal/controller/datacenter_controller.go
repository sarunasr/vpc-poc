package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	configurationv1alpha1 "github.com/sarunasr/vpc-poc/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const workspaceAPIVersion = "opentofu.m.upbound.io/v1beta1"
const workspaceRevisionAnnotation = "infra.example.io/infrastructure-revision"

var moduleVersionPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|v?[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?)$`)

type DatacenterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *DatacenterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	workspace := newWorkspaceObject("", "")
	return ctrl.NewControllerManagedBy(mgr).
		For(&configurationv1alpha1.Datacenter{}).
		Watches(&configurationv1alpha1.VirtualMachine{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, object client.Object) []reconcile.Request {
			vm, ok := object.(*configurationv1alpha1.VirtualMachine)
			if !ok || vm.Spec.DatacenterRef == "" {
				return nil
			}
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: vm.Namespace, Name: vm.Spec.DatacenterRef}}}
		})).
		Watches(workspace, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, object client.Object) []reconcile.Request {
			for _, owner := range object.GetOwnerReferences() {
				if owner.Controller != nil && *owner.Controller && owner.Kind == "Datacenter" {
					return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: object.GetNamespace(), Name: owner.Name}}}
				}
			}
			return nil
		})).
		Complete(r)
}

func (r *DatacenterReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var datacenter configurationv1alpha1.Datacenter
	if err := r.Get(ctx, request.NamespacedName, &datacenter); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var virtualMachines configurationv1alpha1.VirtualMachineList
	if err := r.List(ctx, &virtualMachines, client.InNamespace(datacenter.Namespace)); err != nil {
		return ctrl.Result{}, err
	}
	vmInputs, resolved, err := aggregateVirtualMachines(&datacenter, virtualMachines.Items)
	if err != nil {
		setDatacenterCondition(&datacenter, "InfrastructureReady", metav1.ConditionFalse, "InvalidVirtualMachines", err.Error())
		setDatacenterCondition(&datacenter, "Ready", metav1.ConditionFalse, "InvalidVirtualMachines", "Datacenter VM inputs are invalid")
		return r.updateDatacenterStatus(ctx, &datacenter, ctrl.Result{RequeueAfter: time.Minute})
	}
	revision, err := infrastructureRevision(datacenter.Spec.Terraform, vmInputs)
	if err != nil {
		return ctrl.Result{}, err
	}
	datacenter.Status.DesiredInfrastructureRevision = revision

	workspace, err := r.ensureWorkspace(ctx, &datacenter, vmInputs, revision)
	if err != nil {
		setDatacenterCondition(&datacenter, "InfrastructureReady", metav1.ConditionFalse, "WorkspaceUpdateFailed", err.Error())
		setDatacenterCondition(&datacenter, "Ready", metav1.ConditionFalse, "WorkspaceUpdateFailed", "The Crossplane Workspace could not be updated")
		if statusErr := r.updateDatacenterStatusOnly(ctx, &datacenter); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{}, err
	}
	datacenter.Status.WorkspaceGeneration = workspace.GetGeneration()

	syncedGeneration, synced := workspaceConditionGeneration(workspace, "Synced")
	ready := synced && syncedGeneration >= workspace.GetGeneration() && workspaceConditionTrue(workspace, "Ready")
	if ready {
		for name, vm := range resolved {
			if !workspaceVMOutputMatches(workspace, name, vm.infrastructure.VMID, vm.ansibleTarget.Host) {
				ready = false
				break
			}
		}
	}
	if !ready {
		datacenter.Status.AppliedInfrastructureRevision = ""
		setDatacenterCondition(&datacenter, "InfrastructureReady", metav1.ConditionFalse, "WorkspaceReconciliationPending", "Waiting for the current Workspace generation and every declared VM output")
		setDatacenterCondition(&datacenter, "Ready", metav1.ConditionFalse, "WorkspaceReconciliationPending", "Datacenter infrastructure is not ready")
		return r.updateDatacenterStatus(ctx, &datacenter, ctrl.Result{RequeueAfter: 15 * time.Second})
	}

	datacenter.Status.AppliedInfrastructureRevision = revision
	setDatacenterCondition(&datacenter, "InfrastructureReady", metav1.ConditionTrue, "WorkspaceReconciled", "Workspace inputs were reconciled and all declared VM identities and addresses are present")
	setDatacenterCondition(&datacenter, "Ready", metav1.ConditionTrue, "WorkspaceReconciled", "Datacenter infrastructure is ready")
	return r.updateDatacenterStatus(ctx, &datacenter, ctrl.Result{RequeueAfter: time.Minute})
}

type resolvedVirtualMachine struct {
	infrastructure configurationv1alpha1.VirtualMachineInfrastructure
	configuration  configurationv1alpha1.ConfigurationSpec
	ansibleTarget  configurationv1alpha1.TargetSpec
}

func aggregateVirtualMachines(datacenter *configurationv1alpha1.Datacenter, virtualMachines []configurationv1alpha1.VirtualMachine) (map[string]interface{}, map[string]resolvedVirtualMachine, error) {
	owners := make(map[int64]string, len(virtualMachines))
	for i := range virtualMachines {
		vm := &virtualMachines[i]
		if vm.DeletionTimestamp != nil || vm.Spec.DatacenterRef != datacenter.Name {
			continue
		}
		if vm.Spec.VM.VMID <= 0 {
			return nil, nil, fmt.Errorf("VirtualMachine %s/%s must specify vm.vm_id greater than zero", vm.Namespace, vm.Name)
		}
		if owner, exists := owners[vm.Spec.VM.VMID]; exists {
			return nil, nil, fmt.Errorf("VirtualMachines %s and %s in Datacenter %s use the same Proxmox VMID %d", owner, vm.Name, datacenter.Name, vm.Spec.VM.VMID)
		}
		owners[vm.Spec.VM.VMID] = vm.Name
	}

	inputs := make(map[string]interface{})
	resolved := make(map[string]resolvedVirtualMachine)
	for i := range virtualMachines {
		vm := &virtualMachines[i]
		if vm.DeletionTimestamp != nil || vm.Spec.DatacenterRef != datacenter.Name {
			continue
		}
		infrastructure, err := resolveVirtualMachine(datacenter, vm)
		if err != nil {
			return nil, nil, fmt.Errorf("VirtualMachine %s: %w", vm.Name, err)
		}
		target, err := resolveTarget(datacenter.Spec.Ansible.Defaults, vm.Spec.Target, infrastructure.IPv4CIDR)
		if err != nil {
			return nil, nil, fmt.Errorf("VirtualMachine %s target: %w", vm.Name, err)
		}
		reconciliation := resolveReconciliation(datacenter.Spec.Ansible.Reconciliation, vm.Spec.Reconciliation)
		execution := resolveExecution(datacenter.Spec.Ansible.Execution, vm.Spec.Execution)
		spec := configurationv1alpha1.ConfigurationSpec{
			Target: target,
			Ansible: configurationv1alpha1.AnsibleSpec{
				Collections: append([]configurationv1alpha1.CollectionSpec{}, vm.Spec.Ansible.Collections...),
				Roles:       append([]configurationv1alpha1.RoleSpec{}, vm.Spec.Ansible.Roles...),
				Lifecycle:   vm.Spec.Ansible.Lifecycle,
			},
			Reconciliation: reconciliation,
			Execution:      execution,
		}
		inputs[vm.Name] = terraformVMInput(infrastructure)
		resolved[vm.Name] = resolvedVirtualMachine{
			infrastructure: infrastructure,
			configuration:  spec,
			ansibleTarget:  target,
		}
	}
	return inputs, resolved, nil
}

func resolveVirtualMachine(datacenter *configurationv1alpha1.Datacenter, vm *configurationv1alpha1.VirtualMachine) (configurationv1alpha1.VirtualMachineInfrastructure, error) {
	value := vm.Spec.VM
	defaults := datacenter.Spec.VMDefaults
	if value.Name == "" {
		value.Name = vm.Name
	}
	if value.NodeName == "" {
		value.NodeName = datacenter.Spec.Proxmox.DefaultNode
	}
	if value.TemplateID == 0 {
		value.TemplateID = datacenter.Spec.Proxmox.DefaultTemplate
	}
	if value.Cores == 0 {
		value.Cores = defaults.Cores
	}
	if value.Cores == 0 {
		value.Cores = 1
	}
	if value.MemoryMB == 0 {
		value.MemoryMB = defaults.MemoryMB
	}
	if value.MemoryMB == 0 {
		value.MemoryMB = 1024
	}
	if value.DiskSizeGB == 0 {
		value.DiskSizeGB = defaults.DiskSizeGB
	}
	if value.DiskSizeGB == 0 {
		value.DiskSizeGB = 8
	}
	if value.Bridge == "" {
		value.Bridge = defaults.Bridge
	}
	if value.Bridge == "" {
		value.Bridge = "vmbr0"
	}
	if value.Gateway == "" {
		value.Gateway = defaults.Gateway
	}
	if value.Gateway == "" {
		value.Gateway = "10.10.10.1"
	}
	if len(value.DNSServers) == 0 {
		value.DNSServers = append([]string(nil), defaults.DNSServers...)
	}
	if len(value.DNSServers) == 0 {
		value.DNSServers = []string{"1.1.1.1"}
	}
	if value.CloudInitUser == "" {
		value.CloudInitUser = defaults.CloudInitUser
	}
	if value.CloudInitUser == "" {
		value.CloudInitUser = "debian"
	}
	if value.VMID <= 0 {
		return value, fmt.Errorf("vm.vm_id must be greater than zero")
	}
	if value.Name == "" || value.NodeName == "" || value.TemplateID <= 0 || value.IPv4CIDR == "" {
		return value, fmt.Errorf("vm.name, vm.node_name, vm.template_id and vm.ipv4_cidr (or datacenter defaults) are required")
	}
	if _, _, err := net.ParseCIDR(value.IPv4CIDR); err != nil {
		return value, fmt.Errorf("vm.ipv4_cidr %q is not a valid CIDR: %w", value.IPv4CIDR, err)
	}
	if value.Cores < 1 || value.MemoryMB < 1 || value.DiskSizeGB < 1 {
		return value, fmt.Errorf("cores, memory_mb and disk_size_gb must be greater than zero")
	}
	return value, nil
}

func terraformVMInput(vm configurationv1alpha1.VirtualMachineInfrastructure) map[string]interface{} {
	return map[string]interface{}{
		"name":            vm.Name,
		"vm_id":           vm.VMID,
		"node_name":       vm.NodeName,
		"template_id":     vm.TemplateID,
		"cores":           vm.Cores,
		"memory_mb":       vm.MemoryMB,
		"disk_size_gb":    vm.DiskSizeGB,
		"bridge":          vm.Bridge,
		"ipv4_cidr":       vm.IPv4CIDR,
		"gateway":         vm.Gateway,
		"dns_servers":     vm.DNSServers,
		"cloud_init_user": vm.CloudInitUser,
	}
}

func resolveTarget(defaults, overrides configurationv1alpha1.VirtualMachineTargetSpec, ipv4CIDR string) (configurationv1alpha1.TargetSpec, error) {
	host := overrides.Host
	if host == "" {
		var ip net.IP
		if parsed, _, err := net.ParseCIDR(ipv4CIDR); err == nil {
			ip = parsed
		}
		if ip == nil {
			return configurationv1alpha1.TargetSpec{}, fmt.Errorf("cannot determine target host from vm.ipv4_cidr")
		}
		host = ip.String()
	}
	user := overrides.User
	if user == "" {
		user = defaults.User
	}
	secret := overrides.SSHSecretRef
	if secret == "" {
		secret = defaults.SSHSecretRef
	}
	if user == "" || secret == "" {
		return configurationv1alpha1.TargetSpec{}, fmt.Errorf("SSH user and sshSecretRef must be set on the VirtualMachine or Datacenter defaults")
	}
	proxy := overrides.ProxyJump
	if proxy == nil {
		proxy = defaults.ProxyJump
	}
	result := configurationv1alpha1.TargetSpec{Host: host, User: user, SSHSecretRef: secret, Port: overrides.Port}
	if proxy != nil {
		result.ProxyJump = new(configurationv1alpha1.ProxyJumpSpec)
		*result.ProxyJump = *proxy
	}
	if result.Port == 0 {
		result.Port = defaults.Port
	}
	return result, nil
}

func resolveReconciliation(defaults *configurationv1alpha1.DatacenterReconciliationSpec, overrides *configurationv1alpha1.VirtualMachineReconciliationSpec) configurationv1alpha1.ReconciliationSpec {
	result := configurationv1alpha1.ReconciliationSpec{OnChange: true}
	if defaults != nil {
		if defaults.OnChange != nil {
			result.OnChange = *defaults.OnChange
		}
		if defaults.Schedule != nil {
			result.Schedule = new(configurationv1alpha1.ScheduleSpec)
			*result.Schedule = *defaults.Schedule
		}
	}
	if overrides != nil {
		if overrides.OnChange != nil {
			result.OnChange = *overrides.OnChange
		}
		if overrides.Schedule != nil {
			result.Schedule = new(configurationv1alpha1.ScheduleSpec)
			*result.Schedule = *overrides.Schedule
		}
	}
	return result
}

func resolveExecution(defaults, overrides *configurationv1alpha1.ExecutionSpec) configurationv1alpha1.ExecutionSpec {
	result := configurationv1alpha1.ExecutionSpec{MaxConcurrent: 2, Timeout: metav1.Duration{Duration: 30 * time.Minute}, MaxRetries: 3}
	if defaults != nil {
		if defaults.MaxConcurrent != 0 {
			result.MaxConcurrent = defaults.MaxConcurrent
		}
		if defaults.Timeout.Duration != 0 {
			result.Timeout = defaults.Timeout
		}
		if defaults.MaxRetries != 0 {
			result.MaxRetries = defaults.MaxRetries
		}
	}
	if overrides != nil {
		if overrides.MaxConcurrent != 0 {
			result.MaxConcurrent = overrides.MaxConcurrent
		}
		if overrides.Timeout.Duration != 0 {
			result.Timeout = overrides.Timeout
		}
		if overrides.MaxRetries != 0 {
			result.MaxRetries = overrides.MaxRetries
		}
	}
	return result
}

func infrastructureRevision(terraform configurationv1alpha1.DatacenterTerraformSpec, vms map[string]interface{}) (string, error) {
	input := struct {
		Terraform DatacenterTerraformRevision `json:"terraform"`
		VMs       map[string]interface{}      `json:"vms"`
	}{
		Terraform: DatacenterTerraformRevision{
			WorkspaceName: terraform.WorkspaceName, Module: terraform.Module,
			ProviderConfigRef: terraform.ProviderConfigRef,
			ProxmoxAPIToken:   terraform.ProxmoxAPIToken, SSHPublicKeys: terraform.SSHPublicKeys,
		},
		VMs: vms,
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal datacenter inputs: %w", err)
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

type DatacenterTerraformRevision struct {
	WorkspaceName     string                                    `json:"workspaceName"`
	Module            configurationv1alpha1.TerraformModuleSpec `json:"module"`
	ProviderConfigRef configurationv1alpha1.WorkspaceReference  `json:"providerConfigRef"`
	ProxmoxAPIToken   configurationv1alpha1.SecretKeyReference  `json:"proxmoxAPIToken"`
	SSHPublicKeys     configurationv1alpha1.SecretKeyReference  `json:"sshPublicKeys"`
}

func pinnedModuleReference(module configurationv1alpha1.TerraformModuleSpec) (string, error) {
	if module.Source == "" || !moduleVersionPattern.MatchString(module.Version) {
		return "", fmt.Errorf("Terraform module source and immutable version (40-character commit SHA or pinned semantic version) are required")
	}
	if strings.Contains(module.Source, "ref=") {
		return "", fmt.Errorf("Terraform module source must not include ref=; use module.version")
	}
	separator := "?"
	if strings.Contains(module.Source, "?") {
		separator = "&"
	}
	return module.Source + separator + "ref=" + url.QueryEscape(module.Version), nil
}

func (r *DatacenterReconciler) ensureWorkspace(ctx context.Context, datacenter *configurationv1alpha1.Datacenter, vms map[string]interface{}, revision string) (*unstructured.Unstructured, error) {
	module, err := pinnedModuleReference(datacenter.Spec.Terraform.Module)
	if err != nil {
		return nil, err
	}
	providerName := datacenter.Spec.Terraform.ProviderConfigRef.Name
	if providerName == "" {
		providerName = "default"
	}
	providerKind := datacenter.Spec.Terraform.ProviderConfigRef.Kind
	if providerKind == "" {
		providerKind = "ProviderConfig"
	}
	forProvider := map[string]interface{}{
		"source":           "Remote",
		"module":           module,
		"entrypoint":       datacenter.Spec.Terraform.Module.Entrypoint,
		"remotePullPolicy": "Always",
		"varmap":           map[string]interface{}{"vms": vms},
		"env": []interface{}{
			map[string]interface{}{
				"name": "TF_VAR_proxmox_api_token",
				"secretKeyRef": map[string]interface{}{
					"name": datacenter.Spec.Terraform.ProxmoxAPIToken.Name,
					"key":  datacenter.Spec.Terraform.ProxmoxAPIToken.Key,
				},
			},
			map[string]interface{}{
				"name": "TF_VAR_ssh_public_keys",
				"secretKeyRef": map[string]interface{}{
					"name": datacenter.Spec.Terraform.SSHPublicKeys.Name,
					"key":  datacenter.Spec.Terraform.SSHPublicKeys.Key,
				},
			},
		},
	}
	spec := map[string]interface{}{
		"providerConfigRef":  map[string]interface{}{"kind": providerKind, "name": providerName},
		"managementPolicies": []interface{}{"Observe", "Create", "Update"},
		"forProvider":        forProvider,
	}
	desired := newWorkspaceObject(datacenter.Namespace, datacenter.Spec.Terraform.WorkspaceName)
	desired.Object["spec"] = spec
	desired.SetAnnotations(map[string]string{workspaceRevisionAnnotation: revision})
	if err := controllerutil.SetControllerReference(datacenter, desired, r.Scheme); err != nil {
		return nil, err
	}

	var result *unstructured.Unstructured
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := newWorkspaceObject(datacenter.Namespace, datacenter.Spec.Terraform.WorkspaceName)
		err := r.Get(ctx, types.NamespacedName{Namespace: datacenter.Namespace, Name: datacenter.Spec.Terraform.WorkspaceName}, current)
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, desired.DeepCopy()); err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
			return r.Get(ctx, types.NamespacedName{Namespace: datacenter.Namespace, Name: datacenter.Spec.Terraform.WorkspaceName}, desired)
		}
		if err != nil {
			return err
		}
		if owner := metav1.GetControllerOf(current); owner != nil && owner.UID != datacenter.UID {
			return fmt.Errorf("Workspace %s/%s is already controlled by %s %s", current.GetNamespace(), current.GetName(), owner.Kind, owner.Name)
		}
		base := current.DeepCopy()
		current.Object["spec"] = runtime.DeepCopyJSONValue(spec).(map[string]interface{})
		annotations := current.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[workspaceRevisionAnnotation] = revision
		current.SetAnnotations(annotations)
		if err := controllerutil.SetControllerReference(datacenter, current, r.Scheme); err != nil {
			return err
		}
		if equality.Semantic.DeepEqual(base.Object["spec"], current.Object["spec"]) &&
			equality.Semantic.DeepEqual(base.GetAnnotations(), current.GetAnnotations()) &&
			equality.Semantic.DeepEqual(base.GetOwnerReferences(), current.GetOwnerReferences()) {
			result = current
			return nil
		}
		if err := r.Update(ctx, current); err != nil {
			return err
		}
		result = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = desired
	}
	return result, nil
}

func newWorkspaceObject(namespace, name string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": workspaceAPIVersion,
		"kind":       "Workspace",
		"metadata": map[string]interface{}{
			"namespace": namespace,
			"name":      name,
		},
	}}
	return object
}

func setDatacenterCondition(datacenter *configurationv1alpha1.Datacenter, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{Type: conditionType, Status: status, Reason: reason, Message: message, ObservedGeneration: datacenter.Generation}
	for i := range datacenter.Status.Conditions {
		if datacenter.Status.Conditions[i].Type == conditionType {
			condition.LastTransitionTime = datacenter.Status.Conditions[i].LastTransitionTime
			if datacenter.Status.Conditions[i].Status != status {
				condition.LastTransitionTime = metav1.Now()
			}
			datacenter.Status.Conditions[i] = condition
			return
		}
	}
	condition.LastTransitionTime = metav1.Now()
	datacenter.Status.Conditions = append(datacenter.Status.Conditions, condition)
	sort.Slice(datacenter.Status.Conditions, func(i, j int) bool {
		return datacenter.Status.Conditions[i].Type < datacenter.Status.Conditions[j].Type
	})
}

func (r *DatacenterReconciler) updateDatacenterStatus(ctx context.Context, datacenter *configurationv1alpha1.Datacenter, result ctrl.Result) (ctrl.Result, error) {
	if err := r.updateDatacenterStatusOnly(ctx, datacenter); err != nil {
		return ctrl.Result{}, err
	}
	return result, nil
}

func (r *DatacenterReconciler) updateDatacenterStatusOnly(ctx context.Context, datacenter *configurationv1alpha1.Datacenter) error {
	var latest configurationv1alpha1.Datacenter
	if err := r.Get(ctx, types.NamespacedName{Namespace: datacenter.Namespace, Name: datacenter.Name}, &latest); err != nil {
		return client.IgnoreNotFound(err)
	}
	if equality.Semantic.DeepEqual(latest.Status, datacenter.Status) {
		return nil
	}
	return r.Status().Update(ctx, datacenter)
}
