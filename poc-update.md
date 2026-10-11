# Infrastructure GitOps POC: Refactor to Unified VM Declarations

You are working on an existing Infrastructure GitOps POC.

The current implementation already includes:
- ArgoCD
- Kubernetes / MicroK8s
- Crossplane
- provider-opentofu
- OpenTofu with BPG Proxmox provider
- A custom Go Kubernetes operator
- Ansible Runner
- Kubernetes Jobs for Ansible execution
- Two test Ansible collections
- Configuration CRD
- Ansible reconciliation, scheduling and role lifecycle functionality

The current implementation works, but the user experience is too complicated.

Currently, adding a second VM requires:
1. Editing Terraform main.tf.
2. Updating the Crossplane Workspace Git SHA.
3. Creating a separate Ansible Configuration YAML.
4. Updating several infrastructure-specific settings.

I want to simplify this significantly.

IMPORTANT: Refactor the existing implementation. Do not rewrite the
entire project, replace existing components, or create a separate repository.

Everything must remain in ONE Git repository.

====================================================================
1. MAIN OBJECTIVE
====================================================================

I want to manage each virtual machine using a SINGLE YAML file.

Example:

datacenters/
  dc1/
    datacenter.yaml
    vms/
      web01.yaml
      web02.yaml

  dc2/
    datacenter.yaml
    vms/
      web03.yaml

Adding a new VM should require only:

1. Create web02.yaml.
2. Git commit.
3. Git push.

ArgoCD should synchronize the declaration.

The custom operator should automatically:
- Detect the new VM.
- Update the corresponding Crossplane Workspace.
- Pass the VM parameters to OpenTofu.
- Wait for infrastructure provisioning.
- Execute the required Ansible roles.
- Report deployment status.

Modifying CPU/RAM should trigger infrastructure reconciliation.

Modifying Ansible configuration should trigger Ansible reconciliation.

Deleting a VM declaration should remove that VM through Terraform.

For this isolated POC, automatic VM deletion is acceptable.

Do not introduce complicated deletion approval mechanisms.

Do not introduce additional GitHub Actions workflows, YAML generators,
rendered branches, or ArgoCD Config Management Plugins.

Use Kubernetes CRDs and the existing operator.

====================================================================
2. KEEP THE EXISTING ARCHITECTURE
====================================================================

Keep:

Git
 |
 v
ArgoCD
 |
 v
Kubernetes
 |
 v
Datacenter + VirtualMachine CRDs
 |
 v
Custom Go Operator
 |
 +---------------------------+
 |                           |
 v                           v
Crossplane              Ansible Runner
 |                           |
 v                           v
provider-opentofu       Kubernetes Jobs
 |                           |
 v                           |
OpenTofu                      |
 |                           |
 v                           |
Terraform Providers           |
 |                           |
 +-------------+-------------+
               |
               v
          Configured VM

Do not replace Terraform/OpenTofu with native Crossplane Proxmox resources.

Do not introduce Flux.

Do not introduce AWX.

Do not implement a custom Terraform engine.

Do not replace Ansible Runner.

The custom operator should only coordinate existing components.

====================================================================
3. REPOSITORY STRUCTURE
====================================================================

Refactor the repository toward this structure:

infrastructure-gitops/
|
+-- datacenters/
|   |
|   +-- dc1/
|   |   +-- datacenter.yaml
|   |   +-- vms/
|   |       +-- web01.yaml
|   |       +-- web02.yaml
|   |
|   +-- dc2/
|       +-- datacenter.yaml
|       +-- vms/
|           +-- web03.yaml
|
+-- terraform/
|   +-- modules/
|   |   +-- proxmox-vm/
|   |
|   +-- environments/
|       +-- homelab/
|           +-- main.tf
|           +-- variables.tf
|           +-- provider.tf
|           +-- outputs.tf
|
+-- operator/
|
+-- ansible/
|   +-- collections/
|   +-- execution-environment/
|
+-- crossplane/
|
+-- argocd/
|
+-- docs/

This structure is illustrative.

Preserve existing working components and adjust paths where necessary.

The datacenters directory should contain the user-facing declarations.

Terraform modules and Ansible collections remain reusable components.

====================================================================
4. DATACENTER CRD
====================================================================

Implement a Datacenter CRD.

The Datacenter resource represents a Terraform root module,
its Crossplane Workspace, and common configuration.

Example:

apiVersion: infra.example.io/v1alpha1
kind: Datacenter
metadata:
  name: dc1
  namespace: infrastructure
spec:
  terraform:
    workspaceName: dc1

    module:
      source: git::https://github.com/example/infrastructure-gitops.git//terraform/environments/homelab
      version: v1.0.0

  proxmox:
    defaultNode: pve01
    defaultTemplate: debian13

  ansible:
    defaults:
      user: debian
      sshSecretRef: homelab-vm-ssh

      proxyJump:
        host: 185.170.196.225
        user: root
        sshSecretRef: proxmox-ansible-jump

    reconciliation:
      onChange: true

      schedule:
        type: interval
        duration: 24h

This is a conceptual schema.

Adjust the actual fields to match the existing implementation.

The Datacenter resource must support:
- Terraform root module reference
- Pinned Terraform module revision
- Crossplane Workspace name
- Provider configuration references
- Common VM defaults
- Common Ansible defaults
- SSH and ProxyJump configuration
- Default reconciliation settings

Do not store secret values directly in the Datacenter YAML.

Continue using Kubernetes Secret references.

The operator should create or manage the corresponding
Crossplane Workspace.

For the POC:

1 Datacenter = 1 Crossplane Workspace = 1 Terraform state.

Do not create a separate Workspace for every VM.

Do not make the design depend on specific datacenter names.

====================================================================
5. VIRTUALMACHINE CRD
====================================================================

Implement a VirtualMachine CRD.

Each VirtualMachine should contain both infrastructure and
Ansible configuration.

Example:

apiVersion: infra.example.io/v1alpha1
kind: VirtualMachine

metadata:
  name: web01
  namespace: infrastructure

spec:
  datacenterRef: dc1

  vm:
    name: web01
    target: pve01
    template: debian13

    cores: 2
    memory: 4096

    disks:
      - size: 32
        interface: scsi0

    networks:
      - bridge: vmbr0
        ip: 10.10.10.21/24
        gateway: 10.10.10.1

  ansible:
    collections:
      - name: homelab.web
        version: "1.0.0"

      - name: homelab.system
        version: "1.0.0"

    roles:
      - name: homelab.web.nginx
      - name: homelab.system.textfile

    lifecycle:
      uninstallRemovedRoles: true

  reconciliation:
    onChange: true

    schedule:
      type: cron
      expression: "0 8 * * *"
      timezone: Europe/Vilnius

  execution:
    maxConcurrent: 2
    timeout: 30m
    maxRetries: 3

The VM infrastructure block should be compatible with the
Terraform module's existing input structure.

Prefer using Terraform-compatible field names.

Do not invent a completely different infrastructure schema
if the existing Terraform module already supports the required fields.

The VirtualMachine CRD should allow inheriting common values
from the Datacenter resource.

Explicit VM values override datacenter defaults.

Use deterministic merge semantics and document them.

====================================================================
6. TERRAFORM WORKSPACE AGGREGATION
====================================================================

This is one of the most important requirements.

Our existing production Terraform architecture uses:

1 Terraform root module per datacenter.

Each root module can manage multiple VMs using a map.

For example:

variable "vms" {
  type = map(any)
}

The Terraform module uses:

for_each = var.vms

We want the same architecture in this POC.

The custom operator must:

1. Watch Datacenter resources.
2. Watch VirtualMachine resources.
3. Group VirtualMachine resources by datacenterRef.
4. Read the infrastructure configuration of every VM.
5. Construct one complete vms map for that datacenter.
6. Pass this map to the corresponding Crossplane Workspace.
7. Allow Crossplane/OpenTofu to reconcile the infrastructure.

Example Kubernetes resources:

VirtualMachine/web01:
  datacenterRef: dc1
  cores: 2
  memory: 4096

VirtualMachine/web02:
  datacenterRef: dc1
  cores: 4
  memory: 8192

The operator must construct:

vms = {
  web01 = {
    cores  = 2
    memory = 4096
  }

  web02 = {
    cores  = 4
    memory = 8192
  }
}

This is passed to the Terraform root module through the
Crossplane Workspace variables.

Do not create separate Terraform module declarations manually.

Do not create one Workspace per VM.

The Terraform root module should already contain the necessary
module call using the vms variable.

For example:

module "vms" {
  source = "../../modules/proxmox-vm"

  vms = var.vms
}

Adapt the actual code to the existing module interface.

IMPORTANT:

The operator must always generate a COMPLETE and deterministic
VM map for the Datacenter.

Do not update the Workspace using only the VM that changed.

Do not accidentally remove other VMs from the map.

Use stable VM identifiers as map keys.

Reject duplicate VM names or conflicting VM identifiers.

Handle concurrent VirtualMachine changes safely.

Use Kubernetes resourceVersion optimistic concurrency where appropriate.

Avoid unnecessary Workspace updates when the effective input map
has not changed.

Multiple operator replicas must not race to mutate the same Workspace.

A single logical controller must own the Workspace reconciliation
for each Datacenter.

====================================================================
7. TERRAFORM MODULE VERSIONING
====================================================================

Terraform root module versions must be pinned.

Example:

terraform:
  module:
    source: git::https://github.com/example/infrastructure-gitops.git//terraform/environments/homelab
    version: v1.0.0

The operator should translate this into the appropriate
Crossplane Workspace module reference.

The module reference must resolve to an immutable revision.

IMPORTANT:

VM parameter changes must NOT require manually changing the
Terraform module Git SHA.

CPU, memory, disk, network, and VM additions are passed
through Workspace variables.

The module revision remains unchanged.

The module revision should only change when we explicitly
upgrade the Terraform module itself.

This should work similarly to pinned Ansible collection versions.

If the Terraform module version changes, Crossplane should reconcile
using the new version.

Do not automatically track the latest Terraform Git commit
in the initial POC.

====================================================================
8. INFRASTRUCTURE CHANGE DETECTION
====================================================================

The operator must distinguish infrastructure changes from
Ansible configuration changes.

Examples:

Change CPU:
  Terraform reconciliation only.

Change memory:
  Terraform reconciliation only.

Change disk configuration:
  Terraform reconciliation only.

Change network configuration:
  Terraform reconciliation.
  Run Ansible again only if the target or a relevant dependency changes.

Add a VM:
  Terraform provisioning.
  Then Ansible configuration.

Remove a VM:
  Terraform destruction.

Change Ansible collection version:
  Ansible reconciliation only.

Add an Ansible role:
  Ansible reconciliation only.

Remove an Ansible role:
  Follow uninstallRemovedRoles behavior.

Change Ansible variables:
  Ansible reconciliation only.

Change reconciliation schedule:
  Update scheduling behavior without unnecessarily executing Ansible.

A schedule-only change should not be treated as a configuration
content change unless explicitly required by the scheduling logic.

Implement independent infrastructure and configuration revisions.

For example:

InfrastructureRevision
ConfigurationRevision

Compute deterministic hashes from the effective configuration.

Avoid relying only on Kubernetes resourceVersion or timestamps.

====================================================================
9. INFRASTRUCTURE READINESS
====================================================================

The operator must wait for infrastructure provisioning before
running Ansible on a newly created VM.

Do not treat Workspace Ready=True as sufficient evidence by itself.

A Workspace may still report readiness from an earlier input revision.

The operator must track the desired infrastructure configuration
revision and confirm that the corresponding Workspace reconciliation
has actually completed successfully.

Investigate what information provider-opentofu exposes about
observed generations, successful reconciliation, and applied inputs.

Use the strongest available reliable signal.

Do not implement readiness based on arbitrary sleep delays.

For an individual VM:

1. Confirm its infrastructure is present.
2. Confirm the expected VM identity.
3. Resolve its IP/hostname.
4. Verify SSH reachability.
5. Start Ansible.

The same Workspace may manage multiple VMs.

Therefore, readiness must not accidentally trigger Ansible
for a VM that has not been provisioned yet.

For a VM that has only CPU or RAM changes, do not automatically
rerun Ansible unless the VM identity or configuration dependencies
have changed.

If the VM is recreated, Ansible must run even if the declared
Ansible collection versions remain unchanged.

====================================================================
10. REUSE EXISTING ANSIBLE IMPLEMENTATION
====================================================================

The current project already includes a Configuration CRD and
working Ansible orchestration.

Do not rewrite that subsystem.

Prefer reusing the existing Configuration controller.

The VirtualMachine controller can create or update an internal
Configuration resource derived from VirtualMachine.spec.ansible.

Example:

VirtualMachine/web01
       |
       v
Our Operator
       |
       +--> Crossplane Workspace
       |
       +--> Configuration/web01
                    |
                    v
                Ansible Job
                    |
                    v
               Ansible Runner

The generated Configuration resource must be owned by the
VirtualMachine resource.

ArgoCD must NOT independently manage this generated Configuration.

Avoid having two controllers writing the same fields.

Keep existing SSH credential handling.

Keep existing ProxyJump functionality.

Continue using Kubernetes Secrets.

Reuse existing Ansible Execution Environments and Kubernetes Jobs.

====================================================================
11. KEEP EXISTING ANSIBLE FEATURES
====================================================================

All currently implemented Ansible capabilities must continue working.

Required behavior:

- Pinned collection versions.
- Automatic execution when collection version changes.
- Automatic execution when roles change.
- Automatic execution when role variables change.
- onChange: true/false.
- Cron scheduling.
- Interval scheduling.
- Manual reconciliation.
- Kubernetes Jobs.
- Ansible Runner.
- Execution timeouts.
- Retry policies.
- Execution status.
- Operator restart recovery.
- Role lifecycle metadata.
- uninstallRemovedRoles.

Do not introduce automatic latest collection tracking.

Collection versions remain pinned.

A collection source change without a declared version bump
must not automatically redeploy VMs.

Scheduled Ansible execution must use the pinned collection version.

====================================================================
12. ROLE UNINSTALL
====================================================================

Keep:

ansible:
  lifecycle:
    uninstallRemovedRoles: true

The existing behavior must remain:

If a removed role supports uninstall:
  Execute the previously deployed role with its absent variable.

If a removed role does not support uninstall:
  Skip automatic cleanup and report a warning.

Continue using role metadata:

roles/<role>/meta/gitops.yml

Example:

lifecycle:
  uninstall:
    supported: true
    variable: nginx_state
    value: absent

The existing POC test roles must remain functional:

homelab.web.nginx
homelab.system.textfile

Nginx supports uninstall.

Textfile does not support uninstall.

Do not break the existing lifecycle implementation.

====================================================================
13. DELETING A VM
====================================================================

This is an isolated homelab POC.

Automatic VM destruction is acceptable.

When a VirtualMachine declaration is removed from Git:

1. ArgoCD deletes the VirtualMachine resource.
2. The operator removes that VM from the Datacenter's vms map.
3. The operator updates the Crossplane Workspace.
4. OpenTofu plans and applies the corresponding destroy.
5. The remaining VMs stay managed.

Do not delete or recreate the entire Workspace.

Do not destroy unrelated VMs.

Do not introduce approval workflows for VM deletion.

Do not build an elaborate infrastructure deletion protection system
for this POC.

IMPORTANT:

Ensure deletion events reliably trigger a Datacenter reconciliation.

Avoid finalizer behavior that permanently blocks deletion.

If a finalizer is necessary for correct deletion sequencing,
implement it simply and correctly.

The system must also handle the last VM being removed from a DC.

An empty VM map should have well-defined behavior.

Keep the Workspace and its state unless the Datacenter itself
is explicitly deleted.

Document how Datacenter deletion behaves.

====================================================================
14. DATACENTER DEFAULTS
====================================================================

Avoid repeating common configuration in every VM YAML.

Datacenter should define defaults such as:

- Default Proxmox node
- Default template
- Default SSH user
- SSH Secret
- ProxyJump host
- ProxyJump user
- ProxyJump Secret
- Default Ansible schedule
- Default execution timeout
- Default retry count

VirtualMachine declarations should inherit these defaults.

Example:

Datacenter:
  ansible:
    defaults:
      user: debian
      sshSecretRef: homelab-vm-ssh

      proxyJump:
        host: 185.170.196.225
        user: root
        sshSecretRef: proxmox-ansible-jump

VirtualMachine:
  ansible:
    collections:
      - name: homelab.web
        version: "1.0.0"

    roles:
      - name: homelab.web.nginx

The effective Configuration should contain all necessary values
after inheritance.

Allow VM-specific overrides.

Clearly document inheritance and override rules.

====================================================================
15. ARGOCD INTEGRATION
====================================================================

ArgoCD is already installed.

Do not reinstall it.

Do not introduce Flux.

ArgoCD should watch the datacenter declarations.

For example:

datacenters/dc1/datacenter.yaml
datacenters/dc1/vms/web01.yaml
datacenters/dc1/vms/web02.yaml

Use an ArgoCD Application, Kustomize configuration, or an
appropriate native ArgoCD directory source configuration.

The key requirement is that ArgoCD directly synchronizes the
Datacenter and VirtualMachine CRDs.

Do not introduce a manifest generator.

Do not introduce another rendered branch.

Do not require GitHub Actions to prepare manifests.

A Git push should be sufficient to trigger reconciliation.

Ensure CRDs are installed before ArgoCD applies their resources.

Avoid ArgoCD managing resources generated by the operator.

====================================================================
16. STATUS AND OBSERVABILITY
====================================================================

VirtualMachine resources should expose useful conditions.

Example:

NAME    DATACENTER   INFRA     CONFIG    HEALTH
web01   dc1          Ready     Synced    Healthy
web02   dc1          Ready     Pending   Unknown
web03   dc2          Failed    Pending   Degraded

Suggested status fields:

status:
  infrastructure:
    desiredRevision: "..."
    appliedRevision: "..."
    ready: true

  configuration:
    desiredRevision: "..."
    appliedRevision: "..."
    lastSuccessful: "..."

  conditions:
    - type: InfrastructureReady
      status: "True"

    - type: ConfigurationSynced
      status: "True"

    - type: Ready
      status: "True"

Use proper Kubernetes conditions.

Expose useful failure reasons.

Datacenter should expose Workspace reconciliation status.

Avoid reporting success simply because a Workspace resource exists.

ArgoCD should display meaningful resource health.

Add custom ArgoCD health checks if practical.

No custom UI is required.

====================================================================
17. TESTING
====================================================================

Preserve the existing working VM.

Do not unnecessarily destroy and recreate VM 201 while
refactoring the architecture.

Migrate the existing POC declarative configuration carefully.

Verify that the Terraform state and existing resource addresses
remain stable.

Do not create a second state for the existing VM.

Required tests:

TEST 1: Existing VM

- Existing VM remains functional.
- Terraform state is preserved.
- Existing Ansible configuration still works.

TEST 2: Add second VM

Create:

datacenters/dc1/vms/web02.yaml

Expected:

- ArgoCD synchronizes the resource.
- Operator updates the existing Workspace.
- OpenTofu creates VM 202.
- Existing VM 201 remains unchanged.
- Ansible configures VM 202.
- Nginx serves the expected response.

TEST 3: CPU change

Change web02 CPU.

Expected:

- Workspace variables change.
- OpenTofu reconciles the VM.
- No unnecessary Ansible Job is started.

TEST 4: Ansible version change

Change the pinned collection version.

Expected:

- Ansible Job starts.
- Terraform infrastructure inputs remain unchanged.
- No unnecessary Terraform apply is triggered.

TEST 5: Add role

Add textfile role.

Expected:

- Ansible executes.
- Text file is created.

TEST 6: Remove role

Remove Nginx role.

Expected:

- uninstallRemovedRoles is respected.
- Nginx uninstall executes if supported.
- HTTP endpoint stops responding.

TEST 7: Remove VM

Delete web02.yaml.

Expected:

- ArgoCD removes VirtualMachine/web02.
- Workspace vms map no longer includes web02.
- OpenTofu destroys VM 202.
- VM 201 remains functional.
- Workspace remains available.

TEST 8: Cron

Verify scheduled Ansible execution.

TEST 9: Interval

Verify periodic Ansible execution.

TEST 10: Operator restart

Restart operator during reconciliation.

Verify no duplicate Jobs or unintended Workspace mutations.

TEST 11: Concurrent VM changes

Modify two VM declarations.

Verify the Workspace receives the complete desired VM map.

TEST 12: Empty Datacenter

Remove all VM declarations from a test DC.

Verify the Workspace receives an empty VM map and does not
retain undeclared VMs.

Do not mark tests as passed unless they were actually executed.

====================================================================
18. IMPORTANT IMPLEMENTATION DETAILS
====================================================================

Before implementation:

1. Inspect the existing repository.
2. Identify current Terraform module inputs.
3. Identify the current Workspace CRD schema.
4. Inspect the existing Configuration controller.
5. Inspect ArgoCD Application configuration.
6. Identify the current state backend.
7. Determine how the existing VM can be preserved.
8. Identify required CRD changes.

Then implement the refactor.

Pay particular attention to:

- Stable Terraform resource addresses.
- Existing state preservation.
- Complete VM map reconciliation.
- Workspace update conflicts.
- Controller restart behavior.
- Deletion events.
- Avoiding duplicate Ansible Jobs.
- Correct readiness detection.
- Preventing accidental changes to unrelated VMs.

Use controller-runtime best practices.

Prefer clear and maintainable Go code.

Do not add unnecessary abstractions.

====================================================================
19. DOCUMENTATION
====================================================================

Update the README and architecture documentation.

Explain:

- New Datacenter CRD.
- New VirtualMachine CRD.
- How Datacenter defaults work.
- How one Workspace manages multiple VMs.
- How Terraform receives the vms map.
- How Ansible configuration is derived.
- How Git changes trigger reconciliation.
- How to add a new VM.
- How to change CPU/RAM.
- How to add/remove Ansible roles.
- How to change collection versions.
- How scheduling works.
- How to delete a VM.
- How to troubleshoot Workspace and Ansible Jobs.

The main README should include a complete example.

Adding a second VM should be documented as:

1. Create web02.yaml.
2. Commit.
3. Push.

No manual Terraform edits.

No manual Workspace SHA updates.

No separate Configuration manifests.

====================================================================
20. DEFINITION OF DONE
====================================================================

The refactor is complete when:

[ ] Datacenter CRD exists.

[ ] VirtualMachine CRD exists.

[ ] ArgoCD synchronizes both resource types.

[ ] One Datacenter manages one Crossplane Workspace.

[ ] One Workspace manages multiple VMs.

[ ] The operator aggregates VM infrastructure parameters.

[ ] Terraform receives the complete vms map.

[ ] Terraform module versions remain pinned.

[ ] VM parameter changes do not require Git SHA updates.

[ ] Adding a VM requires only one YAML file.

[ ] Ansible configuration lives in the same YAML.

[ ] Ansible configuration changes trigger Ansible only.

[ ] Infrastructure changes trigger Terraform reconciliation.

[ ] Existing Ansible functionality is preserved.

[ ] Datacenter defaults are supported.

[ ] Removing a VM triggers Terraform destruction.

[ ] Other VMs remain unaffected.

[ ] The existing Terraform state is preserved.

[ ] Operator restart recovery works.

[ ] Tests are executed and documented.

[ ] README reflects the new architecture.

====================================================================
21. FINAL INSTRUCTIONS
====================================================================

This is a POC in my personal homelab.

Prioritize simplicity and functionality.

Do not overengineer security, approval workflows, deletion
protection, or production migration mechanisms.

Do not add GitHub Actions generators.

Do not add Flux.

Do not introduce another Terraform controller.

Do not replace Crossplane provider-opentofu.

Do not rewrite working Ansible functionality.

Keep everything in the existing repository.

IMPORTANT:

Do not just propose the architecture.

Implement the refactor.

Work incrementally and run tests after each major change.

If an existing API limitation prevents the proposed implementation,
explain the limitation and choose the simplest compatible solution.

Preserve the existing working VM and Terraform state where possible.

The final user experience must be:

Create web02.yaml
       |
       v
Git push
       |
       v
ArgoCD
       |
       v
Our Operator
       |
       +--> Crossplane/OpenTofu --> Proxmox
       |
       +--> Ansible Runner --> Configure VM
       |
       v
VM Ready

For existing VMs:

CPU/RAM changes -> Terraform reconciliation.

Ansible version/role changes -> Ansible reconciliation.

Scheduled execution -> Ansible reconciliation.

VM YAML deletion -> Terraform destroy.

This is the desired architecture.

Implement it.