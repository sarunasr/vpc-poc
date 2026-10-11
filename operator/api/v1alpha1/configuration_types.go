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
	Host         string         `json:"host"`
	User         string         `json:"user"`
	SSHSecretRef string         `json:"sshSecretRef"`
	Port         int32          `json:"port,omitempty"`
	ProxyJump    *ProxyJumpSpec `json:"proxyJump,omitempty"`
}

type ProxyJumpSpec struct {
	Host         string `json:"host"`
	User         string `json:"user"`
	SSHSecretRef string `json:"sshSecretRef"`
	Port         int32  `json:"port,omitempty"`
}

type InfrastructureRef struct {
	WorkspaceName          string `json:"workspaceName"`
	InfrastructureRevision string `json:"infrastructureRevision,omitempty"`
	IdentityRevision       string `json:"identityRevision,omitempty"`
	WorkspaceGeneration    int64  `json:"workspaceGeneration,omitempty"`
	VMName                 string `json:"vmName,omitempty"`
	VMID                   int64  `json:"vmID,omitempty"`
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=dc
// +kubebuilder:printcolumn:name="Workspace",type=string,JSONPath=".spec.terraform.workspaceName"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
type Datacenter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DatacenterSpec   `json:"spec"`
	Status            DatacenterStatus `json:"status,omitempty"`
}

type DatacenterSpec struct {
	Terraform  DatacenterTerraformSpec `json:"terraform"`
	Proxmox    DatacenterProxmoxSpec   `json:"proxmox,omitempty"`
	VMDefaults VirtualMachineDefaults  `json:"vmDefaults,omitempty"`
	Ansible    DatacenterAnsibleSpec   `json:"ansible,omitempty"`
}

type DatacenterTerraformSpec struct {
	WorkspaceName     string              `json:"workspaceName"`
	Module            TerraformModuleSpec `json:"module"`
	ProviderConfigRef WorkspaceReference  `json:"providerConfigRef,omitempty"`
	ProxmoxAPIToken   SecretKeyReference  `json:"proxmoxAPIToken"`
	SSHPublicKeys     SecretKeyReference  `json:"sshPublicKeys"`
}

type TerraformModuleSpec struct {
	Source     string `json:"source"`
	Version    string `json:"version"`
	Entrypoint string `json:"entrypoint,omitempty"`
}

type WorkspaceReference struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name"`
}

type SecretKeyReference struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type DatacenterProxmoxSpec struct {
	DefaultNode     string `json:"defaultNode,omitempty"`
	DefaultTemplate int64  `json:"defaultTemplate,omitempty"`
}

type VirtualMachineDefaults struct {
	Cores         int32    `json:"cores,omitempty"`
	MemoryMB      int64    `json:"memory_mb,omitempty"`
	DiskSizeGB    int64    `json:"disk_size_gb,omitempty"`
	Bridge        string   `json:"bridge,omitempty"`
	Gateway       string   `json:"gateway,omitempty"`
	DNSServers    []string `json:"dns_servers,omitempty"`
	CloudInitUser string   `json:"cloud_init_user,omitempty"`
}

type DatacenterAnsibleSpec struct {
	Defaults       VirtualMachineTargetSpec      `json:"defaults,omitempty"`
	Reconciliation *DatacenterReconciliationSpec `json:"reconciliation,omitempty"`
	Execution      *ExecutionSpec                `json:"execution,omitempty"`
}

type DatacenterReconciliationSpec struct {
	OnChange *bool         `json:"onChange,omitempty"`
	Schedule *ScheduleSpec `json:"schedule,omitempty"`
}

type VirtualMachineSpec struct {
	DatacenterRef  string                            `json:"datacenterRef"`
	VM             VirtualMachineInfrastructure      `json:"vm"`
	Target         VirtualMachineTargetSpec          `json:"target,omitempty"`
	Ansible        AnsibleSpec                       `json:"ansible"`
	Reconciliation *VirtualMachineReconciliationSpec `json:"reconciliation,omitempty"`
	Execution      *ExecutionSpec                    `json:"execution,omitempty"`
}

type VirtualMachineInfrastructure struct {
	Name          string   `json:"name,omitempty"`
	VMID          int64    `json:"vm_id"`
	NodeName      string   `json:"node_name,omitempty"`
	TemplateID    int64    `json:"template_id,omitempty"`
	Cores         int32    `json:"cores,omitempty"`
	MemoryMB      int64    `json:"memory_mb,omitempty"`
	DiskSizeGB    int64    `json:"disk_size_gb,omitempty"`
	Bridge        string   `json:"bridge,omitempty"`
	IPv4CIDR      string   `json:"ipv4_cidr"`
	Gateway       string   `json:"gateway,omitempty"`
	DNSServers    []string `json:"dns_servers,omitempty"`
	CloudInitUser string   `json:"cloud_init_user,omitempty"`
}

type VirtualMachineTargetSpec struct {
	Host         string         `json:"host,omitempty"`
	User         string         `json:"user,omitempty"`
	SSHSecretRef string         `json:"sshSecretRef,omitempty"`
	Port         int32          `json:"port,omitempty"`
	ProxyJump    *ProxyJumpSpec `json:"proxyJump,omitempty"`
}

type VirtualMachineReconciliationSpec struct {
	OnChange *bool         `json:"onChange,omitempty"`
	Schedule *ScheduleSpec `json:"schedule,omitempty"`
}

type DatacenterStatus struct {
	DesiredInfrastructureRevision string             `json:"desiredInfrastructureRevision,omitempty"`
	AppliedInfrastructureRevision string             `json:"appliedInfrastructureRevision,omitempty"`
	WorkspaceGeneration           int64              `json:"workspaceGeneration,omitempty"`
	Conditions                    []metav1.Condition `json:"conditions,omitempty"`
}

type VirtualMachineStatus struct {
	InfrastructureRevision       string             `json:"infrastructureRevision,omitempty"`
	DesiredConfigurationRevision string             `json:"desiredConfigurationRevision,omitempty"`
	AppliedConfigurationRevision string             `json:"appliedConfigurationRevision,omitempty"`
	LastSuccessfulExecution      *metav1.Time       `json:"lastSuccessfulExecution,omitempty"`
	Conditions                   []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=vm
// +kubebuilder:printcolumn:name="Datacenter",type=string,JSONPath=".spec.datacenterRef"
// +kubebuilder:printcolumn:name="Infra",type=string,JSONPath=".status.conditions[?(@.type=='InfrastructureReady')].status"
// +kubebuilder:printcolumn:name="Config",type=string,JSONPath=".status.conditions[?(@.type=='ConfigurationSynced')].status"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
type VirtualMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              VirtualMachineSpec   `json:"spec"`
	Status            VirtualMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type DatacenterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Datacenter `json:"items"`
}

// +kubebuilder:object:root=true
type VirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Configuration{}, &ConfigurationList{}, &Datacenter{}, &DatacenterList{}, &VirtualMachine{}, &VirtualMachineList{})
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
