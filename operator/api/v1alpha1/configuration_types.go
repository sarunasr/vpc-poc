package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=cfg
// +kubebuilder:printcolumn:name="Desired",type=string,JSONPath=".status.desiredRevision"
// +kubebuilder:printcolumn:name="Applied",type=string,JSONPath=".status.appliedRevision"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ConfigurationSpec   `json:"spec"`
	Status            ConfigurationStatus `json:"status,omitempty"`
}

type ConfigurationSpec struct {
	Target         TargetSpec         `json:"target"`
	Infrastructure *InfrastructureRef `json:"infrastructureRef,omitempty"`
	Ansible        AnsibleSpec        `json:"ansible"`
	Reconciliation ReconciliationSpec `json:"reconciliation,omitempty"`
	Execution      ExecutionSpec      `json:"execution,omitempty"`
}

type TargetSpec struct {
	Host         string `json:"host"`
	User         string `json:"user"`
	SSHSecretRef string `json:"sshSecretRef"`
	Port         int32  `json:"port,omitempty"`
}

type InfrastructureRef struct {
	WorkspaceName string `json:"workspaceName"`
}

type AnsibleSpec struct {
	Collections []CollectionSpec `json:"collections"`
	Roles       []RoleSpec       `json:"roles"`
	Lifecycle   LifecycleSpec    `json:"lifecycle,omitempty"`
}

type CollectionSpec struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type RoleSpec struct {
	Name string                 `json:"name"`
	Vars map[string]interface{} `json:"vars,omitempty"`
}

type LifecycleSpec struct {
	UninstallRemovedRoles bool `json:"uninstallRemovedRoles,omitempty"`
}

type ReconciliationSpec struct {
	OnChange bool          `json:"onChange"`
	Schedule *ScheduleSpec `json:"schedule,omitempty"`
}

type ScheduleSpec struct {
	Type       string `json:"type"`
	Duration   string `json:"duration,omitempty"`
	Expression string `json:"expression,omitempty"`
	Timezone   string `json:"timezone,omitempty"`
}

type ExecutionSpec struct {
	MaxConcurrent int32           `json:"maxConcurrent,omitempty"`
	Timeout       metav1.Duration `json:"timeout,omitempty"`
	MaxRetries    int32           `json:"maxRetries,omitempty"`
}

type ConfigurationStatus struct {
	DesiredRevision         string             `json:"desiredRevision,omitempty"`
	AppliedRevision         string             `json:"appliedRevision,omitempty"`
	AttemptedRevision       string             `json:"attemptedRevision,omitempty"`
	LastAttemptedExecution  *metav1.Time       `json:"lastAttemptedExecution,omitempty"`
	LastSuccessfulExecution *metav1.Time       `json:"lastSuccessfulExecution,omitempty"`
	LastVerificationTime    *metav1.Time       `json:"lastVerificationTime,omitempty"`
	LastCompletedExecution  *metav1.Time       `json:"lastCompletedExecution,omitempty"`
	LastExecutionResult     string             `json:"lastExecutionResult,omitempty"`
	LastExecutionError      string             `json:"lastExecutionError,omitempty"`
	LastManualRequest       string             `json:"lastManualRequest,omitempty"`
	ActiveJob               string             `json:"activeJob,omitempty"`
	Attempt                 int32              `json:"attempt,omitempty"`
	AppliedRoles            []string           `json:"appliedRoles,omitempty"`
	AppliedCollections      []CollectionSpec   `json:"appliedCollections,omitempty"`
	Conditions              []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
type ConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Configuration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Configuration{}, &ConfigurationList{})
}

func (in *Configuration) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(Configuration)
	in.DeepCopyInto(out)
	return out
}

func (in *ConfigurationList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(ConfigurationList)
	in.DeepCopyInto(out)
	return out
}
