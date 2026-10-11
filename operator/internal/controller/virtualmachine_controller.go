package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	configurationv1alpha1 "github.com/sarunasr/vpc-poc/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type VirtualMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *VirtualMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configurationv1alpha1.VirtualMachine{}).
		Owns(&configurationv1alpha1.Configuration{}).
		Watches(&configurationv1alpha1.Datacenter{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
			datacenter, ok := object.(*configurationv1alpha1.Datacenter)
			if !ok {
				return nil
			}
			var virtualMachines configurationv1alpha1.VirtualMachineList
			if err := r.List(ctx, &virtualMachines, client.InNamespace(datacenter.Namespace)); err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "unable to list VirtualMachines for Datacenter event", "datacenter", datacenter.Name)
				return nil
			}
			requests := make([]reconcile.Request, 0)
			for i := range virtualMachines.Items {
				vm := &virtualMachines.Items[i]
				if vm.Spec.DatacenterRef == datacenter.Name {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: vm.Namespace, Name: vm.Name}})
				}
			}
			return requests
		})).
		Complete(r)
}

func (r *VirtualMachineReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var vm configurationv1alpha1.VirtualMachine
	if err := r.Get(ctx, request.NamespacedName, &vm); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var datacenter configurationv1alpha1.Datacenter
	if err := r.Get(ctx, types.NamespacedName{Namespace: vm.Namespace, Name: vm.Spec.DatacenterRef}, &datacenter); err != nil {
		if apierrors.IsNotFound(err) {
			setVirtualMachineCondition(&vm, "InfrastructureReady", metav1.ConditionFalse, "DatacenterNotFound", fmt.Sprintf("Datacenter %q does not exist", vm.Spec.DatacenterRef))
			setVirtualMachineCondition(&vm, "ConfigurationSynced", metav1.ConditionUnknown, "InfrastructureNotReady", "Ansible waits for a valid Datacenter")
			setVirtualMachineCondition(&vm, "Ready", metav1.ConditionFalse, "DatacenterNotFound", "The referenced Datacenter is not available")
			return r.updateVirtualMachineStatus(ctx, &vm, ctrl.Result{RequeueAfter: time.Minute})
		}
		return ctrl.Result{}, err
	}

	var vmList configurationv1alpha1.VirtualMachineList
	if err := r.List(ctx, &vmList, client.InNamespace(vm.Namespace)); err != nil {
		return ctrl.Result{}, err
	}
	_, resolved, err := aggregateVirtualMachines(&datacenter, vmList.Items)
	if err != nil {
		setVirtualMachineCondition(&vm, "InfrastructureReady", metav1.ConditionFalse, "InvalidVirtualMachines", err.Error())
		setVirtualMachineCondition(&vm, "ConfigurationSynced", metav1.ConditionUnknown, "InfrastructureNotReady", "Ansible waits for valid VM infrastructure")
		setVirtualMachineCondition(&vm, "Ready", metav1.ConditionFalse, "InvalidVirtualMachines", "VirtualMachine infrastructure is invalid")
		return r.updateVirtualMachineStatus(ctx, &vm, ctrl.Result{RequeueAfter: time.Minute})
	}
	effective, ok := resolved[vm.Name]
	if !ok {
		setVirtualMachineCondition(&vm, "InfrastructureReady", metav1.ConditionFalse, "DatacenterReferenceMismatch", "VirtualMachine is not included in the referenced Datacenter map")
		setVirtualMachineCondition(&vm, "ConfigurationSynced", metav1.ConditionUnknown, "InfrastructureNotReady", "Ansible waits for VM infrastructure")
		setVirtualMachineCondition(&vm, "Ready", metav1.ConditionFalse, "InfrastructureNotReady", "VM infrastructure is not ready")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
	infraRevision, err := virtualMachineInfrastructureRevision(datacenter.Spec.Terraform.Module, effective.infrastructure)
	if err != nil {
		return ctrl.Result{}, err
	}
	identityRevision, err := virtualMachineIdentityRevision(datacenter.Spec.Terraform.Module, effective.infrastructure)
	if err != nil {
		return ctrl.Result{}, err
	}
	vm.Status.InfrastructureRevision = infraRevision

	if datacenter.Status.AppliedInfrastructureRevision == "" ||
		datacenter.Status.AppliedInfrastructureRevision != datacenter.Status.DesiredInfrastructureRevision ||
		datacenter.Status.WorkspaceGeneration == 0 {
		setVirtualMachineCondition(&vm, "InfrastructureReady", metav1.ConditionFalse, "WorkspaceReconciliationPending", "Waiting for the Datacenter Workspace to reconcile the complete VM map")
		setVirtualMachineCondition(&vm, "ConfigurationSynced", metav1.ConditionUnknown, "InfrastructureNotReady", "Ansible waits for the VM infrastructure")
		setVirtualMachineCondition(&vm, "Ready", metav1.ConditionFalse, "InfrastructureNotReady", "VM infrastructure is not ready")
		return r.updateVirtualMachineStatus(ctx, &vm, ctrl.Result{RequeueAfter: 15 * time.Second})
	}

	workspace := newWorkspaceObject(vm.Namespace, datacenter.Spec.Terraform.WorkspaceName)
	if err := r.Get(ctx, types.NamespacedName{Namespace: vm.Namespace, Name: datacenter.Spec.Terraform.WorkspaceName}, workspace); err != nil {
		if apierrors.IsNotFound(err) {
			setVirtualMachineCondition(&vm, "InfrastructureReady", metav1.ConditionFalse, "WorkspaceNotFound", "The Datacenter Workspace has not been created")
			setVirtualMachineCondition(&vm, "ConfigurationSynced", metav1.ConditionUnknown, "InfrastructureNotReady", "Ansible waits for the Datacenter Workspace")
			setVirtualMachineCondition(&vm, "Ready", metav1.ConditionFalse, "InfrastructureNotReady", "VM infrastructure is not ready")
			return r.updateVirtualMachineStatus(ctx, &vm, ctrl.Result{RequeueAfter: 15 * time.Second})
		}
		return ctrl.Result{}, err
	}
	generation, synced := workspaceConditionGeneration(workspace, "Synced")
	if workspace.GetGeneration() != datacenter.Status.WorkspaceGeneration ||
		workspace.GetAnnotations()[workspaceRevisionAnnotation] != datacenter.Status.AppliedInfrastructureRevision ||
		!synced || generation < workspace.GetGeneration() ||
		!workspaceConditionTrue(workspace, "Ready") ||
		!workspaceVMOutputMatches(workspace, vm.Name, effective.infrastructure.VMID, effective.ansibleTarget.Host) {
		setVirtualMachineCondition(&vm, "InfrastructureReady", metav1.ConditionFalse, "WorkspaceReconciliationPending", "Workspace status or VM identity/address output does not match the current desired inputs")
		setVirtualMachineCondition(&vm, "ConfigurationSynced", metav1.ConditionUnknown, "InfrastructureNotReady", "Ansible waits for the VM infrastructure")
		setVirtualMachineCondition(&vm, "Ready", metav1.ConditionFalse, "InfrastructureNotReady", "VM infrastructure is not ready")
		return r.updateVirtualMachineStatus(ctx, &vm, ctrl.Result{RequeueAfter: 15 * time.Second})
	}
	setVirtualMachineCondition(&vm, "InfrastructureReady", metav1.ConditionTrue, "WorkspaceReconciled", "The current Workspace generation reports the expected VM identity and address")

	effective.configuration.Infrastructure = &configurationv1alpha1.InfrastructureRef{
		WorkspaceName:          datacenter.Spec.Terraform.WorkspaceName,
		InfrastructureRevision: datacenter.Status.AppliedInfrastructureRevision,
		IdentityRevision:       identityRevision,
		WorkspaceGeneration:    datacenter.Status.WorkspaceGeneration,
		VMName:                 vm.Name,
		VMID:                   effective.infrastructure.VMID,
	}
	if err := r.ensureConfiguration(ctx, &vm, effective.configuration); err != nil {
		return ctrl.Result{}, err
	}
	var configuration configurationv1alpha1.Configuration
	if err := r.Get(ctx, types.NamespacedName{Namespace: vm.Namespace, Name: vm.Name}, &configuration); err != nil {
		return ctrl.Result{}, err
	}
	vm.Status.DesiredConfigurationRevision = configuration.Status.DesiredRevision
	vm.Status.AppliedConfigurationRevision = configuration.Status.AppliedRevision
	if configuration.Status.LastSuccessfulExecution != nil {
		vm.Status.LastSuccessfulExecution = configuration.Status.LastSuccessfulExecution.DeepCopy()
	} else {
		vm.Status.LastSuccessfulExecution = nil
	}
	if condition := findCondition(configuration.Status.Conditions, "ConfigurationSynced"); condition != nil {
		setVirtualMachineCondition(&vm, "ConfigurationSynced", condition.Status, condition.Reason, condition.Message)
	} else {
		setVirtualMachineCondition(&vm, "ConfigurationSynced", metav1.ConditionUnknown, "ConfigurationPending", "Generated Configuration has not completed an Ansible execution")
	}
	if condition := findCondition(configuration.Status.Conditions, "Ready"); condition != nil && condition.Status == metav1.ConditionTrue {
		setVirtualMachineCondition(&vm, "Ready", metav1.ConditionTrue, "ConfigurationApplied", "Infrastructure and Ansible configuration are ready")
	} else if condition != nil {
		setVirtualMachineCondition(&vm, "Ready", metav1.ConditionFalse, condition.Reason, condition.Message)
	} else {
		setVirtualMachineCondition(&vm, "Ready", metav1.ConditionFalse, "ConfigurationPending", "Waiting for the generated Configuration to complete")
	}
	return r.updateVirtualMachineStatus(ctx, &vm, ctrl.Result{})
}

func (r *VirtualMachineReconciler) ensureConfiguration(ctx context.Context, vm *configurationv1alpha1.VirtualMachine, spec configurationv1alpha1.ConfigurationSpec) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var configuration configurationv1alpha1.Configuration
		err := r.Get(ctx, types.NamespacedName{Namespace: vm.Namespace, Name: vm.Name}, &configuration)
		if apierrors.IsNotFound(err) {
			configuration = configurationv1alpha1.Configuration{
				ObjectMeta: metav1.ObjectMeta{Name: vm.Name, Namespace: vm.Namespace},
				Spec:       spec,
			}
			if err := controllerutil.SetControllerReference(vm, &configuration, r.Scheme); err != nil {
				return err
			}
			if err := r.Create(ctx, &configuration); err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
			return nil
		}
		if err != nil {
			return err
		}
		if owner := metav1.GetControllerOf(&configuration); owner != nil && owner.UID != vm.UID {
			return fmt.Errorf("Configuration %s/%s is already controlled by %s %s", configuration.Namespace, configuration.Name, owner.Kind, owner.Name)
		}
		base := configuration.DeepCopy()
		configuration.Spec = spec
		if err := controllerutil.SetControllerReference(vm, &configuration, r.Scheme); err != nil {
			return err
		}
		if equality.Semantic.DeepEqual(base.Spec, configuration.Spec) &&
			equality.Semantic.DeepEqual(base.OwnerReferences, configuration.OwnerReferences) {
			return nil
		}
		return r.Update(ctx, &configuration)
	})
}

func virtualMachineInfrastructureRevision(module configurationv1alpha1.TerraformModuleSpec, infrastructure configurationv1alpha1.VirtualMachineInfrastructure) (string, error) {
	raw, err := json.Marshal(struct {
		Module         configurationv1alpha1.TerraformModuleSpec          `json:"module"`
		Infrastructure configurationv1alpha1.VirtualMachineInfrastructure `json:"infrastructure"`
	}{Module: module, Infrastructure: infrastructure})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

func virtualMachineIdentityRevision(module configurationv1alpha1.TerraformModuleSpec, infrastructure configurationv1alpha1.VirtualMachineInfrastructure) (string, error) {
	type vmIdentity struct {
		Name       string `json:"name"`
		VMID       int64  `json:"vm_id"`
		NodeName   string `json:"node_name"`
		TemplateID int64  `json:"template_id"`
	}
	raw, err := json.Marshal(struct {
		Module   configurationv1alpha1.TerraformModuleSpec `json:"module"`
		Identity vmIdentity                                `json:"identity"`
	}{
		Module: module,
		Identity: vmIdentity{
			Name: infrastructure.Name, VMID: infrastructure.VMID,
			NodeName: infrastructure.NodeName, TemplateID: infrastructure.TemplateID,
		},
	})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

func setVirtualMachineCondition(vm *configurationv1alpha1.VirtualMachine, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{Type: conditionType, Status: status, Reason: reason, Message: message, ObservedGeneration: vm.Generation}
	for i := range vm.Status.Conditions {
		if vm.Status.Conditions[i].Type == conditionType {
			condition.LastTransitionTime = vm.Status.Conditions[i].LastTransitionTime
			if vm.Status.Conditions[i].Status != status {
				condition.LastTransitionTime = metav1.Now()
			}
			vm.Status.Conditions[i] = condition
			return
		}
	}
	condition.LastTransitionTime = metav1.Now()
	vm.Status.Conditions = append(vm.Status.Conditions, condition)
}

func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func (r *VirtualMachineReconciler) updateVirtualMachineStatus(ctx context.Context, vm *configurationv1alpha1.VirtualMachine, result ctrl.Result) (ctrl.Result, error) {
	var latest configurationv1alpha1.VirtualMachine
	if err := r.Get(ctx, types.NamespacedName{Namespace: vm.Namespace, Name: vm.Name}, &latest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if equality.Semantic.DeepEqual(latest.Status, vm.Status) {
		return result, nil
	}
	if err := r.Status().Update(ctx, vm); err != nil {
		return ctrl.Result{}, err
	}
	return result, nil
}
