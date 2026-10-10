package controller

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	configurationv1alpha1 "github.com/sarunasr/vpc-poc/operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDesiredRevisionIsStableAcrossMapOrdering(t *testing.T) {
	first := configurationv1alpha1.ConfigurationSpec{
		Target: configurationv1alpha1.TargetSpec{Host: "10.0.0.1", User: "debian"},
		Ansible: configurationv1alpha1.AnsibleSpec{Roles: []configurationv1alpha1.RoleSpec{{
			Name: "homelab.system.textfile",
			Vars: map[string]interface{}{"content": "hello", "path": "/opt/hello"},
		}}},
	}
	second := first
	second.Ansible.Roles = append([]configurationv1alpha1.RoleSpec(nil), first.Ansible.Roles...)
	second.Ansible.Roles[0].Vars = map[string]interface{}{"path": "/opt/hello", "content": "hello"}
	firstRevision, err := desiredRevision(first)
	if err != nil {
		t.Fatal(err)
	}
	secondRevision, err := desiredRevision(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstRevision != secondRevision {
		t.Fatalf("map ordering changed revision: %s != %s", firstRevision, secondRevision)
	}
}

func TestDesiredRevisionChangesWithDesiredRole(t *testing.T) {
	spec := configurationv1alpha1.ConfigurationSpec{
		Ansible: configurationv1alpha1.AnsibleSpec{Roles: []configurationv1alpha1.RoleSpec{{Name: "homelab.web.nginx"}}},
	}
	first, err := desiredRevision(spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Ansible.Roles[0].Vars = map[string]interface{}{"nginx_page_content": "changed"}
	second, err := desiredRevision(spec)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("desired role variables did not change configuration revision")
	}
}

func TestDesiredRevisionChangesWithProxyJump(t *testing.T) {
	spec := configurationv1alpha1.ConfigurationSpec{
		Target: configurationv1alpha1.TargetSpec{
			Host: "10.0.0.1", User: "debian", SSHSecretRef: "vm-ssh",
		},
	}
	directRevision, err := desiredRevision(spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Target.ProxyJump = &configurationv1alpha1.ProxyJumpSpec{
		Host: "bastion.example.test", User: "jump", SSHSecretRef: "jump-ssh",
	}
	jumpRevision, err := desiredRevision(spec)
	if err != nil {
		t.Fatal(err)
	}
	if directRevision == jumpRevision {
		t.Fatal("proxy jump changes did not change configuration revision")
	}
}

func TestExecutionNameChangesAcrossConfigurationGenerations(t *testing.T) {
	revision := strings.Repeat("a", 64)
	first := executionName("web01", revision, "change-"+revision, 7, 1)
	second := executionName("web01", revision, "change-"+revision, 8, 1)
	if first == second {
		t.Fatalf("same revision in different Configuration generations reused Job name %q", first)
	}
}

func TestScheduleIntervalRunsAtMostOneMissedOccurrence(t *testing.T) {
	config := configurationv1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))},
		Spec: configurationv1alpha1.ConfigurationSpec{
			Reconciliation: configurationv1alpha1.ReconciliationSpec{
				Schedule: &configurationv1alpha1.ScheduleSpec{Type: "interval", Duration: "24h"},
			},
		},
	}
	due, _, key := scheduleDue(config, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC))
	if !due || key != "2026-01-02T00:00:00Z" {
		t.Fatalf("expected a single missed occurrence, got due=%t key=%q", due, key)
	}
}

func TestScheduleCronUsesConfiguredTimezone(t *testing.T) {
	location, err := time.LoadLocation("Europe/Vilnius")
	if err != nil {
		t.Fatal(err)
	}
	config := configurationv1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(time.Date(2026, 10, 10, 4, 0, 0, 0, location))},
		Spec: configurationv1alpha1.ConfigurationSpec{
			Reconciliation: configurationv1alpha1.ReconciliationSpec{
				Schedule: &configurationv1alpha1.ScheduleSpec{
					Type: "cron", Expression: "0 8 * * *", Timezone: "Europe/Vilnius",
				},
			},
		},
	}
	now := time.Date(2026, 10, 10, 8, 1, 0, 0, location)
	due, _, key := scheduleDue(config, now)
	want := time.Date(2026, 10, 10, 8, 0, 0, 0, location).Format(time.RFC3339)
	if !due || key != want {
		t.Fatalf("expected local cron occurrence %q, got due=%t key=%q", want, due, key)
	}
}

func TestJobInputsAreJSONCompatibleYAMLAndIncludeSSHSettings(t *testing.T) {
	config := &configurationv1alpha1.Configuration{
		Spec: configurationv1alpha1.ConfigurationSpec{
			Target: configurationv1alpha1.TargetSpec{Host: "target.example.test", User: "debian", SSHSecretRef: "vm-ssh"},
			Ansible: configurationv1alpha1.AnsibleSpec{
				Roles: []configurationv1alpha1.RoleSpec{{Name: "homelab.web.nginx"}},
			},
		},
	}
	playbook, inventory, _, err := jobInputs(config, "")
	if err != nil {
		t.Fatal(err)
	}
	var plays []map[string]interface{}
	if err := json.Unmarshal([]byte(playbook), &plays); err != nil {
		t.Fatalf("playbook is not valid JSON/YAML: %v", err)
	}
	var hosts map[string]interface{}
	if err := json.Unmarshal([]byte(inventory), &hosts); err != nil {
		t.Fatalf("inventory is not valid JSON/YAML: %v", err)
	}
	if len(plays) != 1 || len(plays[0]["roles"].([]interface{})) != 1 {
		t.Fatalf("unexpected playbook: %s", playbook)
	}
	if !strings.Contains(inventory, `"ansible_host":"target.example.test"`) ||
		!strings.Contains(inventory, "UserKnownHostsFile=/tmp/known_hosts") ||
		!strings.Contains(inventory, "/credentials/ssh-privatekey") {
		t.Fatalf("inventory is missing target or SSH settings: %s", inventory)
	}
}

func TestJobInputsAndJobUseSeparateProxyJumpKey(t *testing.T) {
	config := &configurationv1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{Name: "web01", Namespace: "infrastructure"},
		Spec: configurationv1alpha1.ConfigurationSpec{
			Target: configurationv1alpha1.TargetSpec{
				Host: "10.10.10.21", User: "debian", SSHSecretRef: "vm-ssh",
				ProxyJump: &configurationv1alpha1.ProxyJumpSpec{
					Host: "185.170.196.225", User: "root", SSHSecretRef: "jump-ssh",
				},
			},
			Ansible: configurationv1alpha1.AnsibleSpec{
				Roles: []configurationv1alpha1.RoleSpec{{Name: "homelab.web.nginx"}},
			},
		},
	}
	_, inventory, _, err := jobInputs(config, "")
	if err != nil {
		t.Fatal(err)
	}
	var hosts map[string]interface{}
	if err := json.Unmarshal([]byte(inventory), &hosts); err != nil {
		t.Fatalf("inventory is not valid JSON/YAML: %v", err)
	}
	inventoryText := string(inventory)
	if !strings.Contains(inventoryText, "ProxyCommand=ssh -F /dev/null") ||
		!strings.Contains(inventoryText, "-i /credentials/jump/ssh-privatekey") ||
		!strings.Contains(inventoryText, "-o UserKnownHostsFile=/tmp/known_hosts") ||
		!strings.Contains(inventoryText, "root@185.170.196.225") ||
		!strings.Contains(inventoryText, `"ansible_ssh_private_key_file":"/credentials/ssh-privatekey"`) {
		t.Fatalf("inventory is missing separate target and jump SSH settings: %s", inventory)
	}

	job := buildJob(config, "web01-job", "0123456789abcdef", "runner:latest")
	pod := job.Spec.Template.Spec
	if len(pod.InitContainers) != 1 ||
		!strings.Contains(pod.InitContainers[0].Args[0], "install -m 0600 /ssh-source/target/ssh-privatekey") ||
		!strings.Contains(pod.InitContainers[0].Args[0], "install -m 0600 /ssh-source/jump/ssh-privatekey") {
		t.Fatalf("job does not prepare the jump key with target key: %+v", pod.InitContainers)
	}
	secrets := map[string]string{}
	for _, volume := range pod.Volumes {
		if volume.Secret != nil {
			secrets[volume.Name] = volume.Secret.SecretName
		}
	}
	if secrets["ssh-key-input"] != "vm-ssh" || secrets["jump-key-input"] != "jump-ssh" {
		t.Fatalf("job does not mount separate VM and jump key Secrets: %v", secrets)
	}
}

func TestJobWithoutProxyJumpDoesNotMountJumpSecret(t *testing.T) {
	config := &configurationv1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{Name: "web01", Namespace: "infrastructure"},
		Spec: configurationv1alpha1.ConfigurationSpec{
			Target: configurationv1alpha1.TargetSpec{Host: "10.10.10.21", User: "debian", SSHSecretRef: "vm-ssh"},
		},
	}
	job := buildJob(config, "web01-job", "0123456789abcdef", "runner:latest")
	for _, volume := range job.Spec.Template.Spec.Volumes {
		if volume.Name == "jump-key-input" {
			t.Fatal("direct SSH job unexpectedly mounts a jump-host Secret")
		}
	}
}

func TestBuildJobStoresFullRevisionForAppliedStatus(t *testing.T) {
	revision := strings.Repeat("a", 64)
	config := &configurationv1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{Name: "web01", Namespace: "infrastructure"},
		Spec: configurationv1alpha1.ConfigurationSpec{
			Target: configurationv1alpha1.TargetSpec{Host: "10.10.10.21"},
		},
	}
	job := buildJob(config, "web01-job", revision, "runner:latest")
	if got := job.Labels[revisionLabel]; got != revision[:12] {
		t.Fatalf("revision label = %q, want 12-character prefix", got)
	}
	if got := job.Annotations[revisionAnnotation]; got != revision {
		t.Fatalf("full revision annotation = %q, want %q", got, revision)
	}
	appliedRevision, err := appliedJobRevision(job, revision)
	if err != nil {
		t.Fatal(err)
	}
	if appliedRevision != revision {
		t.Fatalf("applied revision = %q, want full revision %q", appliedRevision, revision)
	}
}

func TestAppliedJobRevisionSupportsLegacyMatchingJob(t *testing.T) {
	revision := strings.Repeat("b", 64)
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:   "legacy-job",
		Labels: map[string]string{revisionLabel: revision[:12]},
	}}
	appliedRevision, err := appliedJobRevision(job, revision)
	if err != nil {
		t.Fatal(err)
	}
	if appliedRevision != revision {
		t.Fatalf("legacy applied revision = %q, want full revision %q", appliedRevision, revision)
	}
}

func TestAppliedJobRevisionRejectsUnmatchedLegacyJob(t *testing.T) {
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:   "legacy-job",
		Labels: map[string]string{revisionLabel: strings.Repeat("a", 12)},
	}}
	if _, err := appliedJobRevision(job, strings.Repeat("b", 64)); err == nil {
		t.Fatal("expected unmatched legacy job revision to be rejected")
	}
}

func TestJobInputsRejectUnsafeProxyJumpValues(t *testing.T) {
	config := &configurationv1alpha1.Configuration{
		Spec: configurationv1alpha1.ConfigurationSpec{
			Target: configurationv1alpha1.TargetSpec{
				ProxyJump: &configurationv1alpha1.ProxyJumpSpec{
					Host: "bastion;touch /tmp/unexpected", User: "root", SSHSecretRef: "jump-ssh",
				},
			},
		},
	}
	if _, _, _, err := jobInputs(config, ""); err == nil {
		t.Fatal("expected unsafe proxy jump host to be rejected")
	}
}

func TestRemovedNginxRoleUsesSupportedUninstallMetadata(t *testing.T) {
	config := &configurationv1alpha1.Configuration{
		Spec: configurationv1alpha1.ConfigurationSpec{
			Target: configurationv1alpha1.TargetSpec{Host: "10.0.0.5", User: "debian", SSHSecretRef: "vm-ssh"},
			Ansible: configurationv1alpha1.AnsibleSpec{
				Collections: []configurationv1alpha1.CollectionSpec{{Name: "homelab.web", Version: "1.0.0"}},
				Lifecycle:   configurationv1alpha1.LifecycleSpec{UninstallRemovedRoles: true},
			},
		},
		Status: configurationv1alpha1.ConfigurationStatus{
			AppliedRoles:       []string{"homelab.web.nginx"},
			AppliedCollections: []configurationv1alpha1.CollectionSpec{{Name: "homelab.web", Version: "1.0.0"}},
		},
	}
	playbook, _, warnings, err := jobInputs(config, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected cleanup warnings: %v", warnings)
	}
	var plays []struct {
		Roles []struct {
			Role string                 `json:"role"`
			Vars map[string]interface{} `json:"vars"`
		} `json:"roles"`
	}
	if err := json.Unmarshal([]byte(playbook), &plays); err != nil {
		t.Fatal(err)
	}
	if len(plays) != 1 || len(plays[0].Roles) != 1 ||
		plays[0].Roles[0].Role != "homelab.web.nginx" ||
		plays[0].Roles[0].Vars["nginx_state"] != "absent" {
		t.Fatalf("supported uninstall was not rendered: %s", playbook)
	}
}

func TestRemovedTextfileRoleIsSkippedWhenUninstallUnsupported(t *testing.T) {
	config := &configurationv1alpha1.Configuration{
		Spec: configurationv1alpha1.ConfigurationSpec{
			Ansible: configurationv1alpha1.AnsibleSpec{
				Collections: []configurationv1alpha1.CollectionSpec{{Name: "homelab.system", Version: "1.0.0"}},
				Lifecycle:   configurationv1alpha1.LifecycleSpec{UninstallRemovedRoles: true},
			},
		},
		Status: configurationv1alpha1.ConfigurationStatus{
			AppliedRoles:       []string{"homelab.system.textfile"},
			AppliedCollections: []configurationv1alpha1.CollectionSpec{{Name: "homelab.system", Version: "1.0.0"}},
		},
	}
	playbook, _, warnings, err := jobInputs(config, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "uninstall is not supported") {
		t.Fatalf("expected unsupported cleanup warning, got %v", warnings)
	}
	var plays []map[string]interface{}
	if err := json.Unmarshal([]byte(playbook), &plays); err != nil {
		t.Fatal(err)
	}
	if len(plays) != 1 || len(plays[0]["roles"].([]interface{})) != 0 {
		t.Fatalf("unsupported role cleanup should not execute: %s", playbook)
	}
}
