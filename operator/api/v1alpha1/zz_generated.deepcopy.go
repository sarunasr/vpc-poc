package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (in *Configuration) DeepCopy() *Configuration {
	if in == nil {
		return nil
	}
	out := new(Configuration)
	in.DeepCopyInto(out)
	return out
}

func (in *Configuration) DeepCopyInto(out *Configuration) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec.Target = in.Spec.Target
	if in.Spec.Target.ProxyJump != nil {
		out.Spec.Target.ProxyJump = new(ProxyJumpSpec)
		*out.Spec.Target.ProxyJump = *in.Spec.Target.ProxyJump
	}
	if in.Spec.Infrastructure != nil {
		out.Spec.Infrastructure = new(InfrastructureRef)
		*out.Spec.Infrastructure = *in.Spec.Infrastructure
	}
	out.Spec.Ansible.Collections = append([]CollectionSpec(nil), in.Spec.Ansible.Collections...)
	out.Spec.Ansible.Roles = make([]RoleSpec, len(in.Spec.Ansible.Roles))
	for i := range in.Spec.Ansible.Roles {
		out.Spec.Ansible.Roles[i] = in.Spec.Ansible.Roles[i]
		if in.Spec.Ansible.Roles[i].Vars != nil {
			out.Spec.Ansible.Roles[i].Vars = make(map[string]interface{}, len(in.Spec.Ansible.Roles[i].Vars))
			for key, value := range in.Spec.Ansible.Roles[i].Vars {
				out.Spec.Ansible.Roles[i].Vars[key] = runtime.DeepCopyJSONValue(value)
			}
		}
	}
	out.Spec.Ansible.Lifecycle = in.Spec.Ansible.Lifecycle
	out.Spec.Reconciliation = in.Spec.Reconciliation
	if in.Spec.Reconciliation.Schedule != nil {
		out.Spec.Reconciliation.Schedule = new(ScheduleSpec)
		*out.Spec.Reconciliation.Schedule = *in.Spec.Reconciliation.Schedule
	}
	out.Spec.Execution = in.Spec.Execution
	in.Status.DeepCopyInto(&out.Status)
}

func (in *ConfigurationStatus) DeepCopyInto(out *ConfigurationStatus) {
	*out = *in
	if in.LastAttemptedExecution != nil {
		out.LastAttemptedExecution = in.LastAttemptedExecution.DeepCopy()
	}
	if in.LastSuccessfulExecution != nil {
		out.LastSuccessfulExecution = in.LastSuccessfulExecution.DeepCopy()
	}
	if in.LastVerificationTime != nil {
		out.LastVerificationTime = in.LastVerificationTime.DeepCopy()
	}
	if in.LastCompletedExecution != nil {
		out.LastCompletedExecution = in.LastCompletedExecution.DeepCopy()
	}
	out.AppliedRoles = append([]string(nil), in.AppliedRoles...)
	out.AppliedCollections = append([]CollectionSpec(nil), in.AppliedCollections...)
	out.Conditions = append([]metav1.Condition(nil), in.Conditions...)
}

func (in *ConfigurationList) DeepCopyInto(out *ConfigurationList) {
	*out = *in
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	out.Items = make([]Configuration, len(in.Items))
	for i := range in.Items {
		in.Items[i].DeepCopyInto(&out.Items[i])
	}
}

func (in *Datacenter) DeepCopy() *Datacenter {
	if in == nil {
		return nil
	}
	out := new(Datacenter)
	in.DeepCopyInto(out)
	return out
}

func (in *Datacenter) DeepCopyInto(out *Datacenter) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec.VMDefaults.DNSServers = append([]string(nil), in.Spec.VMDefaults.DNSServers...)
	if in.Spec.Ansible.Defaults.ProxyJump != nil {
		out.Spec.Ansible.Defaults.ProxyJump = new(ProxyJumpSpec)
		*out.Spec.Ansible.Defaults.ProxyJump = *in.Spec.Ansible.Defaults.ProxyJump
	}
	if in.Spec.Ansible.Reconciliation != nil {
		out.Spec.Ansible.Reconciliation = new(DatacenterReconciliationSpec)
		*out.Spec.Ansible.Reconciliation = *in.Spec.Ansible.Reconciliation
		if in.Spec.Ansible.Reconciliation.OnChange != nil {
			out.Spec.Ansible.Reconciliation.OnChange = new(bool)
			*out.Spec.Ansible.Reconciliation.OnChange = *in.Spec.Ansible.Reconciliation.OnChange
		}
		if in.Spec.Ansible.Reconciliation.Schedule != nil {
			out.Spec.Ansible.Reconciliation.Schedule = new(ScheduleSpec)
			*out.Spec.Ansible.Reconciliation.Schedule = *in.Spec.Ansible.Reconciliation.Schedule
		}
	}
	if in.Spec.Ansible.Execution != nil {
		out.Spec.Ansible.Execution = new(ExecutionSpec)
		*out.Spec.Ansible.Execution = *in.Spec.Ansible.Execution
	}
	out.Status.Conditions = append([]metav1.Condition(nil), in.Status.Conditions...)
}

func (in *Datacenter) DeepCopyObject() runtime.Object {
	if out := in.DeepCopy(); out != nil {
		return out
	}
	return nil
}

func (in *DatacenterList) DeepCopy() *DatacenterList {
	if in == nil {
		return nil
	}
	out := new(DatacenterList)
	in.DeepCopyInto(out)
	return out
}

func (in *DatacenterList) DeepCopyInto(out *DatacenterList) {
	*out = *in
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	out.Items = make([]Datacenter, len(in.Items))
	for i := range in.Items {
		in.Items[i].DeepCopyInto(&out.Items[i])
	}
}

func (in *DatacenterList) DeepCopyObject() runtime.Object {
	if out := in.DeepCopy(); out != nil {
		return out
	}
	return nil
}

func (in *VirtualMachine) DeepCopy() *VirtualMachine {
	if in == nil {
		return nil
	}
	out := new(VirtualMachine)
	in.DeepCopyInto(out)
	return out
}

func (in *VirtualMachine) DeepCopyInto(out *VirtualMachine) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec.VM.DNSServers = append([]string(nil), in.Spec.VM.DNSServers...)
	if in.Spec.Target.ProxyJump != nil {
		out.Spec.Target.ProxyJump = new(ProxyJumpSpec)
		*out.Spec.Target.ProxyJump = *in.Spec.Target.ProxyJump
	}
	out.Spec.Ansible.Collections = append([]CollectionSpec(nil), in.Spec.Ansible.Collections...)
	out.Spec.Ansible.Roles = make([]RoleSpec, len(in.Spec.Ansible.Roles))
	for i := range in.Spec.Ansible.Roles {
		out.Spec.Ansible.Roles[i] = in.Spec.Ansible.Roles[i]
		if in.Spec.Ansible.Roles[i].Vars != nil {
			out.Spec.Ansible.Roles[i].Vars = make(map[string]interface{}, len(in.Spec.Ansible.Roles[i].Vars))
			for key, value := range in.Spec.Ansible.Roles[i].Vars {
				out.Spec.Ansible.Roles[i].Vars[key] = runtime.DeepCopyJSONValue(value)
			}
		}
	}
	if in.Spec.Ansible.Lifecycle != (LifecycleSpec{}) {
		out.Spec.Ansible.Lifecycle = in.Spec.Ansible.Lifecycle
	}
	if in.Spec.Reconciliation != nil {
		out.Spec.Reconciliation = new(VirtualMachineReconciliationSpec)
		*out.Spec.Reconciliation = *in.Spec.Reconciliation
		if in.Spec.Reconciliation.OnChange != nil {
			out.Spec.Reconciliation.OnChange = new(bool)
			*out.Spec.Reconciliation.OnChange = *in.Spec.Reconciliation.OnChange
		}
		if in.Spec.Reconciliation.Schedule != nil {
			out.Spec.Reconciliation.Schedule = new(ScheduleSpec)
			*out.Spec.Reconciliation.Schedule = *in.Spec.Reconciliation.Schedule
		}
	}
	if in.Spec.Execution != nil {
		out.Spec.Execution = new(ExecutionSpec)
		*out.Spec.Execution = *in.Spec.Execution
	}
	if in.Status.LastSuccessfulExecution != nil {
		out.Status.LastSuccessfulExecution = in.Status.LastSuccessfulExecution.DeepCopy()
	}
	out.Status.Conditions = append([]metav1.Condition(nil), in.Status.Conditions...)
}

func (in *VirtualMachine) DeepCopyObject() runtime.Object {
	if out := in.DeepCopy(); out != nil {
		return out
	}
	return nil
}

func (in *VirtualMachineList) DeepCopy() *VirtualMachineList {
	if in == nil {
		return nil
	}
	out := new(VirtualMachineList)
	in.DeepCopyInto(out)
	return out
}

func (in *VirtualMachineList) DeepCopyInto(out *VirtualMachineList) {
	*out = *in
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	out.Items = make([]VirtualMachine, len(in.Items))
	for i := range in.Items {
		in.Items[i].DeepCopyInto(&out.Items[i])
	}
}

func (in *VirtualMachineList) DeepCopyObject() runtime.Object {
	if out := in.DeepCopy(); out != nil {
		return out
	}
	return nil
}
