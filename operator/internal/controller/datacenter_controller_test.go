package controller

import (
	"context"
	"testing"

	configurationv1alpha1 "github.com/sarunasr/vpc-poc/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureWorkspaceAdoptsAndUpdatesCompleteVMMap(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := configurationv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	workspaceGV := schema.GroupVersion{Group: "opentofu.m.upbound.io", Version: "v1beta1"}
	scheme.AddKnownTypeWithName(workspaceGV.WithKind("Workspace"), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(workspaceGV.WithKind("WorkspaceList"), &unstructured.UnstructuredList{})
	metav1.AddToGroupVersion(scheme, workspaceGV)

	datacenter := &configurationv1alpha1.Datacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dc1", Namespace: "infrastructure", UID: types.UID("datacenter-uid"),
		},
		Spec: configurationv1alpha1.DatacenterSpec{Terraform: configurationv1alpha1.DatacenterTerraformSpec{
			WorkspaceName: "homelab",
			Module: configurationv1alpha1.TerraformModuleSpec{
				Source:  "git::ssh://git@github.com/example/repo.git",
				Version: "0123456789abcdef0123456789abcdef01234567",
			},
			ProxmoxAPIToken: configurationv1alpha1.SecretKeyReference{Name: "proxmox", Key: "token"},
			SSHPublicKeys:   configurationv1alpha1.SecretKeyReference{Name: "vm-ssh", Key: "public-keys"},
		}},
	}
	existing := newWorkspaceObject("infrastructure", "homelab")
	existing.SetAnnotations(map[string]string{"argocd.argoproj.io/tracking-id": "root:/Workspace:infrastructure/homelab"})
	existing.Object["status"] = map[string]interface{}{"atProvider": map[string]interface{}{"outputs": map[string]interface{}{"old": "retained only until apply"}}}
	r := &DatacenterReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build(),
		Scheme: scheme,
	}

	fullVMMap := map[string]interface{}{
		"web01": terraformVMInput(configurationv1alpha1.VirtualMachineInfrastructure{
			Name: "gitops-poc-201", VMID: 201, NodeName: "node1", TemplateID: 9000,
			Cores: 1, MemoryMB: 1024, DiskSizeGB: 8, Bridge: "vmbr0",
			IPv4CIDR: "10.10.10.21/24", Gateway: "10.10.10.1",
			DNSServers: []string{"1.1.1.1"}, CloudInitUser: "debian",
		}),
		"web02": terraformVMInput(configurationv1alpha1.VirtualMachineInfrastructure{
			Name: "gitops-poc-202", VMID: 202, NodeName: "node1", TemplateID: 9000,
			Cores: 1, MemoryMB: 1024, DiskSizeGB: 8, Bridge: "vmbr0",
			IPv4CIDR: "10.10.10.22/24", Gateway: "10.10.10.1",
			DNSServers: []string{"1.1.1.1"}, CloudInitUser: "debian",
		}),
	}
	revision := "expected-revision"
	workspace, err := r.ensureWorkspace(ctx, datacenter, fullVMMap, revision)
	if err != nil {
		t.Fatal(err)
	}
	varmap, found, err := unstructured.NestedMap(workspace.Object, "spec", "forProvider", "varmap", "vms")
	if err != nil || !found || len(varmap) != 2 {
		t.Fatalf("Workspace did not receive the complete VM map: %#v, found=%t, err=%v", varmap, found, err)
	}
	web01, ok := varmap["web01"].(map[string]interface{})
	if !ok {
		t.Fatalf("Workspace VM input has unexpected type: %#v", varmap["web01"])
	}
	if cores, ok := web01["cores"].(int64); !ok || cores != 1 {
		t.Fatalf("Workspace VM input did not use a JSON-compatible core count: %#v", web01["cores"])
	}
	dnsServers, ok := web01["dns_servers"].([]interface{})
	if !ok || len(dnsServers) != 1 || dnsServers[0] != "1.1.1.1" {
		t.Fatalf("Workspace VM input did not preserve DNS server list: %#v", web01["dns_servers"])
	}
	if workspace.GetAnnotations()["argocd.argoproj.io/tracking-id"] == "" {
		t.Fatal("Workspace adoption removed Argo CD tracking metadata")
	}
	if workspace.GetAnnotations()[workspaceRevisionAnnotation] != revision {
		t.Fatal("Workspace does not identify its desired infrastructure revision")
	}
	owner := metav1.GetControllerOf(workspace)
	if owner == nil || owner.UID != datacenter.UID {
		t.Fatalf("Workspace was not adopted by the Datacenter: %#v", owner)
	}

	resourceVersion := workspace.GetResourceVersion()
	workspace, err = r.ensureWorkspace(ctx, datacenter, fullVMMap, revision)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.GetResourceVersion() != resourceVersion {
		t.Fatal("unchanged Workspace inputs caused an unnecessary update")
	}

	workspace, err = r.ensureWorkspace(ctx, datacenter, map[string]interface{}{}, "empty-revision")
	if err != nil {
		t.Fatal(err)
	}
	varmap, found, err = unstructured.NestedMap(workspace.Object, "spec", "forProvider", "varmap", "vms")
	if err != nil || !found || len(varmap) != 0 {
		t.Fatalf("empty VM set did not replace the old map: %#v, found=%t, err=%v", varmap, found, err)
	}
}

func TestEnsureWorkspaceRejectsConflictingController(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := configurationv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	workspaceGV := schema.GroupVersion{Group: "opentofu.m.upbound.io", Version: "v1beta1"}
	scheme.AddKnownTypeWithName(workspaceGV.WithKind("Workspace"), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(workspaceGV.WithKind("WorkspaceList"), &unstructured.UnstructuredList{})
	metav1.AddToGroupVersion(scheme, workspaceGV)

	datacenter := &configurationv1alpha1.Datacenter{
		ObjectMeta: metav1.ObjectMeta{Name: "dc1", Namespace: "infrastructure", UID: types.UID("new-owner")},
		Spec: configurationv1alpha1.DatacenterSpec{Terraform: configurationv1alpha1.DatacenterTerraformSpec{
			WorkspaceName:   "homelab",
			Module:          configurationv1alpha1.TerraformModuleSpec{Source: "git::ssh://git@github.com/example/repo.git", Version: "1.0.0"},
			ProxmoxAPIToken: configurationv1alpha1.SecretKeyReference{Name: "proxmox", Key: "token"},
			SSHPublicKeys:   configurationv1alpha1.SecretKeyReference{Name: "vm-ssh", Key: "public-keys"},
		}},
	}
	workspace := newWorkspaceObject("infrastructure", "homelab")
	controller := true
	workspace.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: configurationv1alpha1.GroupVersion.String(), Kind: "Datacenter", Name: "old-dc",
		UID: types.UID("old-owner"), Controller: &controller,
	}})
	r := &DatacenterReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(workspace).Build(),
		Scheme: scheme,
	}

	if _, err := r.ensureWorkspace(ctx, datacenter, map[string]interface{}{}, "revision"); err == nil {
		t.Fatal("expected an error when a different Datacenter controls the Workspace")
	}
}
