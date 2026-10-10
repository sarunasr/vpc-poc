package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

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
