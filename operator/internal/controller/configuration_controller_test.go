package controller

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	configurationv1alpha1 "github.com/sarunasr/vpc-poc/operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestDesiredRevisionIgnoresScheduleAndInfrastructureRevision(t *testing.T) {
	spec := configurationv1alpha1.ConfigurationSpec{
		Target:         configurationv1alpha1.TargetSpec{Host: "10.0.0.1", User: "debian", SSHSecretRef: "vm-ssh"},
		Ansible:        configurationv1alpha1.AnsibleSpec{Roles: []configurationv1alpha1.RoleSpec{{Name: "homelab.web.nginx"}}},
		Reconciliation: configurationv1alpha1.ReconciliationSpec{OnChange: true},
		Execution:      configurationv1alpha1.ExecutionSpec{MaxConcurrent: 2},
		Infrastructure: &configurationv1alpha1.InfrastructureRef{
			WorkspaceName: "dc1", InfrastructureRevision: strings.Repeat("a", 64),
			IdentityRevision:    strings.Repeat("c", 64),
			WorkspaceGeneration: 3, VMName: "web01", VMID: 201,
		},
	}
	first, err := desiredRevision(spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Reconciliation.Schedule = &configurationv1alpha1.ScheduleSpec{Type: "interval", Duration: "24h"}
	spec.Reconciliation.OnChange = false
	spec.Execution.MaxConcurrent = 8
	spec.Infrastructure.InfrastructureRevision = strings.Repeat("b", 64)
	spec.Infrastructure.WorkspaceGeneration = 7
	second, err := desiredRevision(spec)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("schedule, execution limits, or infrastructure generation changed the Ansible revision")
	}
	spec.Infrastructure.IdentityRevision = strings.Repeat("d", 64)
	identityChangedRevision, err := desiredRevision(spec)
	if err != nil {
		t.Fatal(err)
	}
	if second == identityChangedRevision {
		t.Fatal("VM identity revision changes did not trigger configuration reconciliation")
	}
}

func TestAggregateVirtualMachinesBuildsCompleteMapAndMergesDefaults(t *testing.T) {
	datacenter := &configurationv1alpha1.Datacenter{
		ObjectMeta: metav1.ObjectMeta{Name: "dc1"},
		Spec: configurationv1alpha1.DatacenterSpec{
			Proxmox: configurationv1alpha1.DatacenterProxmoxSpec{DefaultNode: "pve01", DefaultTemplate: 9000},
			VMDefaults: configurationv1alpha1.VirtualMachineDefaults{
				Cores: 2, MemoryMB: 4096, DiskSizeGB: 32, Bridge: "vmbr0",
				Gateway: "10.10.10.1", DNSServers: []string{"1.1.1.1"}, CloudInitUser: "debian",
			},
			Ansible: configurationv1alpha1.DatacenterAnsibleSpec{
				Defaults: configurationv1alpha1.VirtualMachineTargetSpec{
					User: "debian", SSHSecretRef: "vm-ssh",
					ProxyJump: &configurationv1alpha1.ProxyJumpSpec{Host: "jump.example.test", User: "root", SSHSecretRef: "jump-ssh"},
				},
				Reconciliation: &configurationv1alpha1.DatacenterReconciliationSpec{
					OnChange: boolPointer(true), Schedule: &configurationv1alpha1.ScheduleSpec{Type: "interval", Duration: "24h"},
				},
			},
		},
	}
	virtualMachines := []configurationv1alpha1.VirtualMachine{
		{ObjectMeta: metav1.ObjectMeta{Name: "web02"}, Spec: configurationv1alpha1.VirtualMachineSpec{
			DatacenterRef: "dc1", VM: configurationv1alpha1.VirtualMachineInfrastructure{VMID: 202, IPv4CIDR: "10.10.10.22/24"},
			Ansible: configurationv1alpha1.AnsibleSpec{Collections: []configurationv1alpha1.CollectionSpec{{Name: "homelab.web", Version: "1.0.0"}}, Roles: []configurationv1alpha1.RoleSpec{{Name: "homelab.web.nginx"}}},
		}},
		{ObjectMeta: metav1.ObjectMeta{Name: "web01"}, Spec: configurationv1alpha1.VirtualMachineSpec{
			DatacenterRef: "dc1", VM: configurationv1alpha1.VirtualMachineInfrastructure{
				Name: "gitops-poc-201", VMID: 201, Cores: 1, IPv4CIDR: "10.10.10.21/24",
			},
			Ansible: configurationv1alpha1.AnsibleSpec{Collections: []configurationv1alpha1.CollectionSpec{{Name: "homelab.web", Version: "1.0.0"}}, Roles: []configurationv1alpha1.RoleSpec{{Name: "homelab.web.nginx"}}},
		}},
	}
	inputs, resolved, err := aggregateVirtualMachines(datacenter, virtualMachines)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 2 || len(resolved) != 2 {
		t.Fatalf("aggregated %d inputs and %d resolved VMs, want two each", len(inputs), len(resolved))
	}
	web01 := inputs["web01"].(map[string]interface{})
	if web01["vm_id"] != int64(201) || web01["name"] != "gitops-poc-201" || web01["cores"] != int64(1) {
		t.Fatalf("web01 overrides were not applied: %#v", web01)
	}
	web02 := inputs["web02"].(map[string]interface{})
	if web02["vm_id"] != int64(202) || web02["node_name"] != "pve01" || web02["template_id"] != int64(9000) ||
		web02["cores"] != int64(2) || web02["memory_mb"] != int64(4096) || web02["ipv4_cidr"] != "10.10.10.22/24" {
		t.Fatalf("web02 defaults were not applied: %#v", web02)
	}
	if resolved["web02"].configuration.Target.Host != "10.10.10.22" ||
		resolved["web02"].configuration.Target.ProxyJump == nil ||
		resolved["web02"].configuration.Reconciliation.Schedule.Duration != "24h" {
		t.Fatalf("Ansible defaults were not inherited: %+v", resolved["web02"].configuration)
	}
	reversed := []configurationv1alpha1.VirtualMachine{virtualMachines[1], virtualMachines[0]}
	reversedInputs, _, err := aggregateVirtualMachines(datacenter, reversed)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(inputs)
	secondJSON, _ := json.Marshal(reversedInputs)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("VM list order changed the effective input map: %s != %s", firstJSON, secondJSON)
	}
}

func TestAggregateVirtualMachinesRejectsDuplicateVMIDsWithinDatacenter(t *testing.T) {
	datacenter := &configurationv1alpha1.Datacenter{
		ObjectMeta: metav1.ObjectMeta{Name: "dc1"},
		Spec: configurationv1alpha1.DatacenterSpec{
			Proxmox:    configurationv1alpha1.DatacenterProxmoxSpec{DefaultNode: "pve01", DefaultTemplate: 9000},
			VMDefaults: configurationv1alpha1.VirtualMachineDefaults{Cores: 1, MemoryMB: 1024, DiskSizeGB: 8, Bridge: "vmbr0", Gateway: "10.0.0.1", CloudInitUser: "debian"},
			Ansible:    configurationv1alpha1.DatacenterAnsibleSpec{Defaults: configurationv1alpha1.VirtualMachineTargetSpec{User: "debian", SSHSecretRef: "vm-ssh"}},
		},
	}
	virtualMachines := []configurationv1alpha1.VirtualMachine{
		{ObjectMeta: metav1.ObjectMeta{Name: "web01"}, Spec: configurationv1alpha1.VirtualMachineSpec{DatacenterRef: "dc1", VM: configurationv1alpha1.VirtualMachineInfrastructure{VMID: 201, IPv4CIDR: "10.0.0.21/24"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "web02"}, Spec: configurationv1alpha1.VirtualMachineSpec{DatacenterRef: "dc1", VM: configurationv1alpha1.VirtualMachineInfrastructure{VMID: 201, IPv4CIDR: "10.0.0.22/24"}}},
	}
	if _, _, err := aggregateVirtualMachines(datacenter, virtualMachines); err == nil {
		t.Fatal("expected duplicate Proxmox VM IDs in one Datacenter to be rejected")
	}
	virtualMachines[1].Spec.DatacenterRef = "dc2"
	if _, _, err := aggregateVirtualMachines(datacenter, virtualMachines); err != nil {
		t.Fatalf("duplicate VM IDs in separate Datacenters should be independent: %v", err)
	}
}

func TestResolveReconciliationDefaultsOnChangeAndAllowsFalseOverride(t *testing.T) {
	schedule := &configurationv1alpha1.ScheduleSpec{Type: "interval", Duration: "24h"}
	defaults := &configurationv1alpha1.DatacenterReconciliationSpec{Schedule: schedule}
	resolved := resolveReconciliation(defaults, nil)
	if !resolved.OnChange || resolved.Schedule == nil || resolved.Schedule.Duration != "24h" {
		t.Fatalf("Datacenter defaults should retain onChange=true and its schedule: %+v", resolved)
	}
	disabled := false
	resolved = resolveReconciliation(defaults, &configurationv1alpha1.VirtualMachineReconciliationSpec{OnChange: &disabled})
	if resolved.OnChange {
		t.Fatal("explicit VirtualMachine onChange=false was ignored")
	}
}

func TestResolveExecutionMergesPartialDefaultsAndOverrides(t *testing.T) {
	defaults := &configurationv1alpha1.ExecutionSpec{Timeout: metav1.Duration{Duration: 10 * time.Minute}}
	resolved := resolveExecution(defaults, nil)
	if resolved.MaxConcurrent != 2 || resolved.Timeout.Duration != 10*time.Minute || resolved.MaxRetries != 3 {
		t.Fatalf("partial Datacenter execution defaults did not preserve built-in defaults: %+v", resolved)
	}
	overrides := &configurationv1alpha1.ExecutionSpec{MaxConcurrent: 4}
	resolved = resolveExecution(defaults, overrides)
	if resolved.MaxConcurrent != 4 || resolved.Timeout.Duration != 10*time.Minute || resolved.MaxRetries != 3 {
		t.Fatalf("VirtualMachine execution overrides did not merge correctly: %+v", resolved)
	}
}

func TestVirtualMachineIdentityRevisionTracksRecreationInputs(t *testing.T) {
	module := configurationv1alpha1.TerraformModuleSpec{Source: "git::ssh://git@example.test/repo.git", Version: "v1.0.0"}
	infrastructure := configurationv1alpha1.VirtualMachineInfrastructure{
		Name: "web01", VMID: 201, NodeName: "pve01", TemplateID: 9000,
		Cores: 1, MemoryMB: 1024, IPv4CIDR: "10.0.0.21/24",
	}
	initial, err := virtualMachineIdentityRevision(module, infrastructure)
	if err != nil {
		t.Fatal(err)
	}
	infrastructure.Cores = 4
	infrastructure.MemoryMB = 8192
	unchanged, err := virtualMachineIdentityRevision(module, infrastructure)
	if err != nil {
		t.Fatal(err)
	}
	if initial != unchanged {
		t.Fatal("CPU or memory changes unexpectedly changed the VM identity revision")
	}
	infrastructure.TemplateID = 9001
	recreated, err := virtualMachineIdentityRevision(module, infrastructure)
	if err != nil {
		t.Fatal(err)
	}
	if initial == recreated {
		t.Fatal("template changes did not change the VM identity revision")
	}
}

func TestPinnedModuleReferenceUsesDeclaredVersion(t *testing.T) {
	module := configurationv1alpha1.TerraformModuleSpec{
		Source: "git::ssh://git@github.com/example/repo.git", Version: "0123456789abcdef0123456789abcdef01234567",
	}
	got, err := pinnedModuleReference(module)
	if err != nil {
		t.Fatal(err)
	}
	if got != module.Source+"?ref="+module.Version {
		t.Fatalf("module reference = %q", got)
	}
	module.Source += "?ref=main"
	if _, err := pinnedModuleReference(module); err == nil {
		t.Fatal("expected source URLs with a preexisting ref to be rejected")
	}
}

func TestWorkspaceVMOutputRequiresMatchingIdentityAndIP(t *testing.T) {
	workspace := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"atProvider": map[string]interface{}{
				"outputs": map[string]interface{}{
					"vms": map[string]interface{}{
						"web01": map[string]interface{}{
							"vm_id":          int64(201),
							"ipv4_addresses": []interface{}{[]interface{}{"127.0.0.1"}, []interface{}{"10.10.10.21"}},
						},
					},
				},
			},
		},
	}}
	if !workspaceVMOutputMatches(workspace, "web01", 201, "10.10.10.21") {
		t.Fatal("expected matching VM identity and IP output to be ready")
	}
	if workspaceVMOutputMatches(workspace, "web01", 202, "10.10.10.21") ||
		workspaceVMOutputMatches(workspace, "web01", 201, "10.10.10.22") ||
		workspaceVMOutputMatches(workspace, "web02", 201, "10.10.10.21") {
		t.Fatal("mismatched VM identity or IP was reported ready")
	}
	if !workspaceVMOutputMatches(workspace, "web01", 201, "web01.example.test") {
		t.Fatal("expected a declared hostname to be ready when the Workspace reports a valid IP")
	}
	workspaceWithoutIP := workspace.DeepCopy()
	unstructured.RemoveNestedField(workspaceWithoutIP.Object, "status", "atProvider", "outputs", "vms", "web01", "ipv4_addresses")
	if workspaceVMOutputMatches(workspaceWithoutIP, "web01", 201, "web01.example.test") {
		t.Fatal("hostname-only readiness was reported without an infrastructure IP output")
	}
}

func TestWorkspaceReadyRequiresCurrentGenerationRevisionAndVMOutput(t *testing.T) {
	scheme := runtime.NewScheme()
	workspaceGV := schema.GroupVersion{Group: "opentofu.m.upbound.io", Version: "v1beta1"}
	scheme.AddKnownTypeWithName(workspaceGV.WithKind("Workspace"), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(workspaceGV.WithKind("WorkspaceList"), &unstructured.UnstructuredList{})
	metav1.AddToGroupVersion(scheme, workspaceGV)

	workspace := workspaceReadyObject("infrastructure", "homelab")
	workspace.SetGeneration(5)
	workspace.SetAnnotations(map[string]string{workspaceRevisionAnnotation: "revision-5"})
	workspace.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{"type": "Synced", "status": "True", "observedGeneration": int64(5)},
			map[string]interface{}{"type": "Ready", "status": "True"},
		},
		"atProvider": map[string]interface{}{
			"outputs": map[string]interface{}{
				"vms": map[string]interface{}{
					"web01": map[string]interface{}{
						"vm_id":          int64(201),
						"ipv4_addresses": []interface{}{[]interface{}{"10.10.10.21"}},
					},
				},
			},
		},
	}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(workspace).Build()
	reconciler := &ConfigurationReconciler{Reader: reader}
	reference := configurationv1alpha1.InfrastructureRef{
		WorkspaceName: "homelab", InfrastructureRevision: "revision-5",
		WorkspaceGeneration: 5, VMName: "web01", VMID: 201,
	}
	ready, err := reconciler.workspaceReady(context.Background(), "infrastructure", reference, "10.10.10.21")
	if err != nil || !ready {
		t.Fatalf("expected current Workspace generation to be ready, ready=%t err=%v", ready, err)
	}

	reference.InfrastructureRevision = "stale-revision"
	ready, err = reconciler.workspaceReady(context.Background(), "infrastructure", reference, "10.10.10.21")
	if err != nil || ready {
		t.Fatalf("stale infrastructure revision reported ready, ready=%t err=%v", ready, err)
	}
	reference.InfrastructureRevision = "revision-5"
	reference.WorkspaceGeneration = 4
	ready, err = reconciler.workspaceReady(context.Background(), "infrastructure", reference, "10.10.10.21")
	if err != nil || ready {
		t.Fatalf("stale Workspace generation reported ready, ready=%t err=%v", ready, err)
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
