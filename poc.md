# Infrastructure GitOps Controller
## Complete POC Implementation Specification

You are tasked with designing and implementing a Kubernetes-native Infrastructure GitOps Controller.

This is a Proof of Concept (POC) for managing infrastructure provisioning and operating system configuration through GitOps.

The initial target is Proxmox virtual machines, but the architecture should remain extensible to other infrastructure providers.

The entire implementation must live in a SINGLE Git repository.

I will provide additional information about my Kubernetes, ArgoCD, and Proxmox environments separately.

Do not assume infrastructure details, credentials, IP addresses, or hostnames that have not been provided.

IMPORTANT:
- Kubernetes is already installed.
- ArgoCD is already installed and operational.
- Proxmox VE is already installed and operational.
- A Debian 13 VM template is already available.
- Do not reinstall existing infrastructure unnecessarily.
- Use current documentation and verify compatibility before implementing components.
- Build a functional POC, not just example manifests.
- Test all implemented functionality.
- Do not implement everything blindly. Validate each architectural component before moving forward.


====================================================================
1. PROJECT OBJECTIVE
====================================================================

We want to create an ArgoCD-like GitOps experience for infrastructure.

Currently, infrastructure deployments commonly involve separate tools:

- Terraform
- Ansible
- AWX
- GitHub Actions
- Proxmox
- NetBox

The purpose of this project is to provide a unified declarative infrastructure lifecycle.

Our desired architecture:

Git Repository
      |
      v
    ArgoCD
      |
      v
 Kubernetes
      |
      v
Infrastructure GitOps Controller
      |
      +---------------------------+
      |                           |
      v                           v
Infrastructure                Configuration
Reconciliation                Reconciliation
      |                           |
      v                           v
  Crossplane               Kubernetes Jobs
      |                           |
      v                           v
provider-opentofu           Ansible Runner
      |                           |
      v                           |
   OpenTofu                       |
      |                           |
      v                           |
Terraform Providers               |
      |                           |
      +-------------+-------------+
                    |
                    v
          Infrastructure Ready
                    |
                    v
            Configuration Applied
                    |
                    v
                Healthy

The operator must coordinate infrastructure and configuration deployment.

Ansible configuration must not start until the required infrastructure is ready.

The system must support continuous reconciliation, not just one-time CI/CD execution.


====================================================================
2. ARCHITECTURAL DECISIONS
====================================================================

We have already discussed and agreed on the following initial architecture.

GitOps:
  ArgoCD

Control plane:
  Kubernetes

Infrastructure reconciliation:
  Crossplane

Terraform execution:
  Crossplane provider-opentofu

Terraform provider:
  BPG Proxmox provider

Configuration management:
  Ansible

Ansible execution:
  Ansible Runner

Ansible execution environment:
  Kubernetes Jobs

Orchestration:
  Custom Kubernetes operator

Operator language:
  Go

Operator framework:
  Kubebuilder / controller-runtime

Infrastructure source:
  Terraform HCL

Configuration source:
  Kubernetes Custom Resources

Source of truth:
  Git

Secrets:
  Kubernetes Secrets for POC
  Extensible to Vault in the future

Deployment status:
  Kubernetes CRD status and conditions


IMPORTANT ARCHITECTURAL PRINCIPLES:

1. Do not implement another Terraform engine.
2. Do not implement another Ansible engine.
3. Use existing infrastructure execution tools.
4. The custom operator should orchestrate existing tools.
5. Crossplane must own Terraform/OpenTofu reconciliation.
6. The custom operator must own Ansible configuration reconciliation.
7. Terraform state must remain persistent.
8. Infrastructure and configuration reconciliation must be separate.
9. Avoid unnecessary coupling to Proxmox.
10. The system must be extensible to arbitrary Terraform providers.

We are NOT using Flux.

ArgoCD is already installed and must remain our GitOps solution.

Do not introduce Flux simply to use Tofu Controller.


====================================================================
3. CROSSPLANE AND TERRAFORM
====================================================================

Our initial candidate is:

https://github.com/upbound/provider-opentofu

The expected infrastructure architecture:

ArgoCD
   |
   v
Crossplane
   |
   v
provider-opentofu
   |
   v
OpenTofu
   |
   v
BPG Proxmox Terraform Provider
   |
   v
Proxmox API

Before implementation:

- Verify the current Crossplane version.
- Verify provider-opentofu compatibility.
- Verify whether the project is actively maintained.
- Inspect the actual Workspace CRD schema.
- Verify OpenTofu execution behavior.
- Verify timeout handling.
- Verify remote state configuration.
- Verify drift detection.
- Verify reconciliation behavior.
- Verify deletion policies.
- Verify failure recovery.
- Verify supported authentication methods.

Do not assume that provider-opentofu is production-ready.

If important limitations exist, document them and investigate alternatives.

However, do not immediately replace it with a custom Terraform executor.

We prefer using an existing, reliable infrastructure controller.


====================================================================
4. ALTERNATIVE PROVISIONING APPROACH
====================================================================

We also want to compare two infrastructure provisioning models.

OPTION A: CROSSPLANE + OPENTOFU

Crossplane
    |
    v
provider-opentofu
    |
    v
OpenTofu CLI
    |
    v
BPG Terraform Provider
    |
    v
Proxmox

Characteristics:

- Existing Terraform modules can be reused.
- Terraform/OpenTofu state is required.
- Arbitrary Terraform providers can be supported.
- Infrastructure is managed using HCL.
- Easier adoption of existing Terraform infrastructure.


OPTION B: CROSSPLANE + NATIVE PROXMOX PROVIDER

Candidate:

https://github.com/valkiriaaquatica/provider-proxmox-bpg

Architecture:

Crossplane
    |
    v
Proxmox BPG Crossplane Provider
    |
    v
Proxmox API

Characteristics:

- Kubernetes-native resource declarations.
- No conventional shared Terraform state backend.
- Managed resources have their own persistent identity and state metadata.
- No Terraform CLI required for VM provisioning.
- Potentially more Kubernetes-native lifecycle management.

For the first implementation:

IMPLEMENT OPTION A FIRST.

Investigate Option B and document the comparison.

If practical, implement a separate experimental example using Option B.

Do not manage the same Proxmox VM simultaneously through both approaches.

Compare:

- Provider maturity
- Supported Proxmox functionality
- VM lifecycle behavior
- Drift detection
- Reconciliation performance
- State management
- Failure recovery
- Resource adoption
- Operational complexity
- Scalability
- Kubernetes integration

The main POC must not depend on Option B being functional.


====================================================================
5. EXISTING TERRAFORM STRUCTURE
====================================================================

Our production infrastructure currently uses a structure similar to:

terraform/
|
+-- modules/
|   |
|   +-- hostinger-netbox/
|   |   +-- main.tf
|   |   +-- variables.tf
|   |   +-- outputs.tf
|   |   +-- provider.tf
|   |
|   +-- hostinger-proxmox-bpg/
|   |   +-- main.tf
|   |   +-- variables.tf
|   |   +-- versions.tf
|   |
|   +-- hostinger-proxmox-bpg-infra/
|       +-- main.tf
|       +-- variables.tf
|       +-- versions.tf
|
+-- prod/
    |
    +-- br-asc/
    |   +-- hostinger_dns.tf
    |   +-- hostinger_hardware.tf
    |   +-- hostinger_hvps.tf
    |   +-- hostinger_infra.tf
    |   +-- hostinger_network.tf
    |   +-- hostinger_observability.tf
    |   +-- hostinger_storage.tf
    |   +-- hostinger_webhosting.tf
    |   +-- provider.tf
    |
    +-- br-cam/
    |   +-- ...
    |
    +-- de-dus/
        +-- ...

Terraform configurations are organized by datacenter.

Multiple .tf files in the same directory belong to the same Terraform root module.

We may have multiple VM resources in a single Terraform state.

The POC does not need to reproduce the full production structure.

However, the architecture must not assume:

1 VM = 1 Terraform state

or:

1 VM = 1 Crossplane Workspace

The future system should support Terraform root modules managing multiple resources.

Terraform execution must not be Proxmox-specific.

A single Terraform root module may use:

- Proxmox provider
- NetBox provider
- DNS provider
- Other Terraform providers

If someone modifies NetBox resources in Terraform, infrastructure reconciliation must detect and apply those changes as well.

Do not create a solution that only recognizes Proxmox VM resources.


====================================================================
6. POC TERRAFORM IMPLEMENTATION
====================================================================

Create a reusable Terraform module:

terraform/modules/proxmox-vm/

Files:

main.tf
variables.tf
outputs.tf
versions.tf

Create an environment:

terraform/environments/homelab/

Files:

main.tf
provider.tf
variables.tf
outputs.tf

The module must support:

- VM creation
- VM CPU configuration
- VM memory configuration
- VM disk configuration
- VM networking
- VM name
- Proxmox node
- Debian 13 template cloning
- Cloud-init
- SSH public keys
- VM IP output where possible

Use the existing Debian 13 template.

Avoid unnecessary VM recreation.

Terraform state must be stored persistently.

Do not store state only in ephemeral containers.

For the POC, select an appropriate OpenTofu remote backend.

Document:

- State location
- State locking
- State recovery
- State isolation
- Backup considerations

Prevent concurrent writers against the same Terraform state.

The default infrastructure deletion policy must be Retain.

Do not automatically destroy VMs when Git declarations disappear unless an explicit destructive policy has been configured and the behavior has been tested.


====================================================================
7. CUSTOM KUBERNETES OPERATOR
====================================================================

Implement a Kubernetes operator in Go using Kubebuilder.

The operator coordinates infrastructure and configuration lifecycle.

It must NOT implement Terraform plan/apply itself.

It must NOT implement Terraform state management.

It must NOT directly replace the BPG provider.

It must NOT reimplement Ansible modules.

Its responsibilities include:

1. Reading declarative configuration.
2. Observing infrastructure readiness.
3. Resolving target hosts.
4. Detecting Ansible configuration changes.
5. Creating Kubernetes Jobs for Ansible execution.
6. Monitoring Job status.
7. Tracking successfully applied configuration revisions.
8. Supporting scheduled reconciliation.
9. Supporting manual reconciliation.
10. Handling failed executions.
11. Detecting removed Ansible roles.
12. Running uninstall logic where supported.
13. Exposing useful Kubernetes status conditions.
14. Avoiding duplicate executions.
15. Recovering after operator restarts.

Use controller-runtime reconciliation patterns.

Avoid creating a Kubernetes Job on every reconciliation.

Use deterministic execution identifiers.

Maintain clear ownership of created Kubernetes resources.

Implement proper finalizer behavior where necessary.

Do not create destructive finalizer behavior by default.


====================================================================
8. CUSTOM RESOURCE DESIGN
====================================================================

For the POC, prefer separating infrastructure provisioning from configuration management.

This should allow us to support physical Proxmox hosts later.

The first implementation only needs virtual machines.

Example conceptual Configuration CRD:

apiVersion: infra.example.io/v1alpha1
kind: Configuration

metadata:
  name: web01
  namespace: infrastructure

spec:
  target:
    host: web01.lab.local
    user: debian

  infrastructureRef:
    name: homelab

  ansible:
    collections:
      - name: homelab.web
        version: "1.0.0"

      - name: homelab.system
        version: "1.0.0"

    roles:
      - homelab.web.nginx
      - homelab.system.textfile

    lifecycle:
      uninstallRemovedRoles: true

  reconciliation:
    onChange: true

    schedule:
      type: cron
      expression: "0 8 * * *"
      timezone: Europe/Vilnius

  execution:
    maxConcurrent: 10
    timeout: 30m
    maxRetries: 3

This schema is conceptual.

Design the actual CRD using Kubebuilder conventions.

Add appropriate validation.

The configuration resource must not become tightly coupled to Proxmox-specific APIs.

For the first POC, a static target hostname or IP address is sufficient.

Keep inventory resolution extensible for future NetBox integration.


====================================================================
9. ANSIBLE EXECUTION
====================================================================

Ansible must execute inside Kubernetes Jobs.

Use:

- Ansible Runner
- Ansible Execution Environments
- Kubernetes Jobs

Architecture:

Our Operator
     |
     v
Kubernetes Job
     |
     v
Ansible Execution Environment
     |
     v
Ansible Runner
     |
     v
Target VM

Each execution should have its own Kubernetes Job.

For example:

10 parallel Ansible executions = 10 Kubernetes Jobs / Pods.

For the POC, do not implement custom persistent worker pools.

The Kubernetes Job must receive:

- Target inventory
- Playbook
- Collection versions
- Role variables
- SSH credentials
- Execution timeout
- Execution identifier

Credentials must not be hardcoded.

Use Kubernetes Secrets initially.

Do not print private keys or passwords in logs.

Capture:

- Job creation timestamp
- Start timestamp
- Completion timestamp
- Exit status
- Applied collection versions
- Execution result
- Relevant logs

Prevent two conflicting Ansible executions against the same host.

Use concurrency limits.

Implement retry handling.

Do not blindly retry indefinitely.

Preserve execution information when the operator restarts.


====================================================================
10. ANSIBLE COLLECTION VERSIONING
====================================================================

For the first POC:

ONLY PINNED COLLECTION VERSIONS.

We previously discussed automatically tracking the latest Git revision.

That feature is postponed.

The initial behavior should resemble Docker image version upgrades.

Example:

ansible:
  collections:
    - name: homelab.web
      version: "1.0.0"

  roles:
    - homelab.web.nginx

When the version changes:

1.0.0 -> 1.0.1

the controller must detect the change and execute Ansible.

If someone changes the Ansible role source code but does not update the declared collection version, no immediate deployment should occur.

Scheduled or manual executions must continue to use the declared pinned artifact.

The operator should track:

DesiredRevision
AppliedRevision

Example:

DesiredRevision: 1.0.1
AppliedRevision: 1.0.0

This means the configuration requires reconciliation.

After successful Ansible execution:

DesiredRevision: 1.0.1
AppliedRevision: 1.0.1

A failed deployment must not update AppliedRevision.

IMPORTANT:

Collection versions must resolve to immutable artifacts or immutable Git commits.

Do not dynamically fetch the latest version during execution.

Because everything is stored in one repository, implement a reproducible collection packaging mechanism.

The two collections should be independently versioned.

In the future, we may add:

revisionPolicy: FollowBranch

This would detect new Git commits and automatically deploy changed roles to all affected VMs.

Do not implement this feature yet.

However, avoid architectural decisions that would make it difficult to introduce later.


====================================================================
11. ANSIBLE RECONCILIATION TRIGGERS
====================================================================

Support three execution trigger types.

--------------------------------------------------
11.1 ON CHANGE
--------------------------------------------------

Example:

reconciliation:
  onChange: true

Execute Ansible when the effective configuration changes.

Examples:

- New role added
- Role removed
- Collection version changed
- Role variables changed
- Target configuration changed

Use a deterministic desired configuration hash or equivalent mechanism.

Do not use timestamps as the only method of detecting changes.

Unrelated Git commits must not automatically trigger Ansible.


--------------------------------------------------
11.2 INTERVAL
--------------------------------------------------

Example:

reconciliation:
  onChange: true

  schedule:
    type: interval
    duration: 24h

Execute the configured Ansible roles periodically.

Use the declared pinned collection versions.

Do not automatically upgrade collections.


--------------------------------------------------
11.3 CRON
--------------------------------------------------

Example:

reconciliation:
  onChange: true

  schedule:
    type: cron
    expression: "0 8 * * *"
    timezone: Europe/Vilnius

This must run every day at 08:00 Lithuanian local time.

Support daylight saving time correctly.

Cron and interval are alternative periodic scheduling modes.

Only one periodic scheduling mode should be active for a Configuration resource.

OnChange may be enabled together with either mode.

Handle controller restarts safely.

Prevent duplicate execution of the same scheduled occurrence.

Define and document the behavior for missed scheduled executions.

A reasonable default is to execute at most one missed occurrence after downtime.


--------------------------------------------------
11.4 MANUAL EXECUTION
--------------------------------------------------

Provide a way to manually trigger reconciliation.

A Kubernetes annotation is sufficient.

Example:

kubectl annotate configuration web01 \
  infra.example.io/reconcile-request="$(date +%s)" \
  -n infrastructure \
  --overwrite

The exact annotation name may differ.

A new manual request should initiate Ansible execution.

Do not execute the same request repeatedly.


====================================================================
12. ANSIBLE ROLE REMOVAL
====================================================================

Ansible does not automatically uninstall resources when a role is removed.

We want optional role lifecycle management.

Use this configuration parameter:

ansible:
  lifecycle:
    uninstallRemovedRoles: true

This is the selected name.

When a role is removed:

1. Compare the previous successfully applied role list with the desired list.
2. Identify removed roles.
3. Check role lifecycle metadata.
4. If uninstall is supported, execute the uninstall procedure.
5. If uninstall is unsupported, skip destructive cleanup.
6. Report the result.
7. Preserve pending cleanup operations until resolved.

If uninstallRemovedRoles is false:

- Removed roles stop being executed.
- No uninstall attempt is made.

If uninstallRemovedRoles is true:

- Supported roles are uninstalled.
- Unsupported roles are skipped with a warning.

Do not assume every Ansible role can be removed automatically.

Do not attempt to infer uninstall behavior by analyzing arbitrary Ansible tasks.


====================================================================
13. CUSTOM ROLE LIFECYCLE METADATA
====================================================================

Create a metadata file inside each role.

Location:

roles/<role>/meta/gitops.yml

Example:

lifecycle:
  uninstall:
    supported: true
    variable: nginx_state
    value: absent

This metadata is custom to our project.

It is not standard Ansible metadata.

The controller must read this metadata from the relevant collection artifact.

For supported roles, the metadata describes which variable activates uninstall behavior.

Example:

- name: Uninstall nginx
  ansible.builtin.include_role:
    name: homelab.web.nginx
  vars:
    nginx_state: absent

For safety, cleanup should use the collection version that was previously successfully deployed.

Do not automatically use the newest collection version for uninstall.

If cleanup fails:

- Preserve the pending cleanup operation.
- Do not report successful removal.
- Retry according to policy.
- Prevent duplicate cleanup Jobs.
- Show a useful error condition.

Do not destroy the VM because Ansible cleanup failed.


====================================================================
14. CREATE TWO REAL ANSIBLE COLLECTIONS
====================================================================

Both collections must exist in the SAME repository.

Create two independently versioned Ansible collections:

1. homelab.web
2. homelab.system

They must follow actual Ansible collection conventions.

Do not create fake collections containing only arbitrary role directories.

Include:

- galaxy.yml
- roles
- defaults
- tasks
- handlers where needed
- metadata
- README
- tests

Create a reproducible process to build collection artifacts.

Use real pinned versions during execution.


====================================================================
15. COLLECTION: homelab.web
====================================================================

Collection name:

homelab.web

Role:

homelab.web.nginx

Location:

ansible/collections/ansible_collections/homelab/web/

The role must install and configure Nginx on Debian 13.

Requirements:

1. Install Nginx.
2. Enable the Nginx service.
3. Start the Nginx service.
4. Create a simple web page.
5. Support configurable web page content.
6. Support clean uninstall.
7. Be idempotent.
8. Include lifecycle metadata.

Default port:

80

Default response:

Web is working!

Verification:

curl -fsS http://VM_IP/

Expected:

Web is working!

The role must support:

nginx_state: present

and:

nginx_state: absent

PRESENT behavior:

- Install Nginx.
- Configure the web root.
- Deploy the test webpage.
- Enable Nginx.
- Start Nginx.

ABSENT behavior:

- Stop Nginx.
- Disable Nginx.
- Remove Nginx package.
- Remove only POC-specific files managed by the role.
- Preserve unrelated files and user data.

Lifecycle metadata:

roles/nginx/meta/gitops.yml

Content:

lifecycle:
  uninstall:
    supported: true
    variable: nginx_state
    value: absent


--------------------------------------------------
15.1 NGINX INSTALL TEST
--------------------------------------------------

Add:

homelab.web.nginx

to the Configuration resource.

Expected:

- Ansible Job starts.
- Nginx installs.
- Service becomes active.
- HTTP endpoint responds successfully.

Test:

curl -fsS http://VM_IP/

Expected:

Web is working!


--------------------------------------------------
15.2 NGINX VERSION UPDATE TEST
--------------------------------------------------

Version 1.0.0:

Web is working!

Update collection content.

Create version 1.0.1.

New output:

Web is working! Version 2

Change the Configuration resource:

1.0.0 -> 1.0.1

Expected:

- Operator detects the changed version.
- A new Kubernetes Job starts.
- Ansible deploys updated content.
- HTTP response changes.
- AppliedRevision is updated.


--------------------------------------------------
15.3 NGINX REMOVAL TEST
--------------------------------------------------

Remove:

homelab.web.nginx

from the desired Configuration.

Keep:

uninstallRemovedRoles: true

Expected:

- Operator detects role removal.
- Role metadata indicates uninstall support.
- Operator executes nginx_state: absent.
- Nginx service stops.
- Nginx package is removed.
- HTTP endpoint is no longer available.

Test:

curl --max-time 5 http://VM_IP/

Expected:

Connection refused or another connection-level failure.

Do not expect HTTP 404 or HTTP 500.

An uninstalled web server normally does not return HTTP responses.

Verify that the failure is caused by removing Nginx, not by a broken network connection.


====================================================================
16. COLLECTION: homelab.system
====================================================================

Collection name:

homelab.system

Role:

homelab.system.textfile

Location:

ansible/collections/ansible_collections/homelab/system/

This role must create a simple text file.

Default destination:

/opt/gitops-demo/hello.txt

Default content:

Hello from Infrastructure GitOps!

Requirements:

1. Create the destination directory.
2. Create the text file.
3. Write configurable content.
4. Be idempotent.
5. Support content updates.
6. Work on Debian 13.
7. NOT support automatic uninstall.

Verification:

cat /opt/gitops-demo/hello.txt

Expected:

Hello from Infrastructure GitOps!

Lifecycle metadata:

roles/textfile/meta/gitops.yml

Content:

lifecycle:
  uninstall:
    supported: false


--------------------------------------------------
16.1 TEXTFILE REMOVAL TEST
--------------------------------------------------

Remove:

homelab.system.textfile

from the Configuration.

Keep:

uninstallRemovedRoles: true

Expected:

- Operator detects role removal.
- Operator checks lifecycle metadata.
- Uninstall is unsupported.
- No destructive cleanup is executed.
- The text file remains.
- The controller reports a clear warning.

This behavior is intentional.

The purpose is to verify that unsupported uninstall operations are handled safely.


====================================================================
17. CONFIGURATION EXECUTION SEMANTICS
====================================================================

The controller must track the effective desired configuration.

Changes that trigger Ansible:

- Collection version changes
- Role list changes
- Role variables change
- Target configuration changes

Track at least:

DesiredRevision
AppliedRevision
LastAttemptedExecution
LastSuccessfulExecution

Example:

DesiredRevision: abc123
AppliedRevision: abc123

Means the latest declared configuration was successfully applied.

Example:

DesiredRevision: def456
AppliedRevision: abc123

Means a new deployment is required or currently pending.

Failed Ansible execution must not advance AppliedRevision.

Successful execution does not guarantee that no configuration drift can occur later.

Expose last verification time separately from applied revision.

When only Terraform CPU or memory changes:

- Do not automatically rerun every Ansible role.
- Unless the VM is recreated or a configured dependency requires it.

Infrastructure readiness and instance identity must be considered.

A newly recreated VM must receive configuration even if the Ansible collection versions did not change.


====================================================================
18. DEPLOYMENT ORDER
====================================================================

For a new VM:

1. ArgoCD synchronizes infrastructure declarations.
2. Crossplane receives the infrastructure Workspace.
3. OpenTofu provisions the VM.
4. Proxmox VM is created.
5. Terraform outputs become available.
6. Crossplane reports infrastructure readiness.
7. Operator resolves VM IP or hostname.
8. Operator waits for target readiness.
9. Operator prepares Ansible execution.
10. Kubernetes Job is created.
11. Ansible Runner executes roles.
12. Operator observes Job completion.
13. Configuration is marked applied.
14. Overall resource status becomes healthy.

Do not assume that Terraform apply completion means SSH is immediately available.

Use bounded readiness checks.

If cloud-init is involved, account for initialization time.

Never start Ansible against an unavailable target.

Do not create unlimited retry loops.


====================================================================
19. DRIFT DETECTION
====================================================================

INFRASTRUCTURE DRIFT:

Crossplane + OpenTofu owns infrastructure drift detection.

Example:

Terraform declares:

CPU = 4

Someone manually changes Proxmox VM CPU to 2.

Expected:

- OpenTofu reconciliation detects drift.
- The controller reports the difference.
- The configured policy determines whether to apply correction.

Verify actual provider behavior during the POC.

Do not assume all resource fields support automatic drift correction.


CONFIGURATION DRIFT:

Ansible is not a complete declarative state engine.

Therefore, support scheduled reconciliation.

Example:

Every day at 08:00:

- Start Ansible Job.
- Use pinned collection versions.
- Reapply configured roles.
- Restore managed configuration where supported.

Example:

The textfile role creates:

/opt/gitops-demo/hello.txt

Someone manually changes the file.

At the next scheduled reconciliation, Ansible should restore the declared content.

Do not claim real-time configuration drift detection.

The POC only needs periodic reconciliation and execution status.


====================================================================
20. JOB CONCURRENCY
====================================================================

Support concurrent Ansible execution using Kubernetes Jobs.

Example:

10 executions
     |
     v
10 Kubernetes Jobs
     |
     v
10 Pods

Use:

maxConcurrent

to restrict parallel execution.

For example:

execution:
  maxConcurrent: 10

Do not create unlimited Jobs.

Prevent multiple simultaneous Jobs for the same target.

Handle:

- Job timeouts
- Job failures
- Retry policies
- Operator restarts
- Kubernetes Job cleanup
- Execution history

For the POC, one Job per target execution is acceptable.

In the future, we may support grouped inventory execution.

Do not implement grouped execution unless necessary for the POC.


====================================================================
21. OBSERVABILITY AND STATUS
====================================================================

Expose meaningful Kubernetes status conditions.

Example:

NAME       INFRA       CONFIG       HEALTH
web01      Ready       Synced       Healthy
web02      Ready       Failed       Degraded
web03      Pending     Pending      Unknown

Suggested fields:

status:
  infrastructure:
    ready: true
    lastReconciled: "..."

  configuration:
    desiredRevision: "..."
    appliedRevision: "..."
    lastAttempted: "..."
    lastSuccessful: "..."

  conditions:
    - type: Ready
      status: "True"

    - type: ConfigurationSynced
      status: "True"

Use standard Kubernetes conditions.

Include meaningful reasons:

- InfrastructureNotReady
- TargetUnreachable
- AnsibleExecutionFailed
- CleanupFailed
- CleanupUnsupported
- ConfigurationApplied

Expose Kubernetes Events for important state changes.

Provide useful controller logs.

Do not create a custom frontend for the POC.

Use Kubernetes resource status and ArgoCD UI.

If needed, implement ArgoCD custom health checks.


====================================================================
22. SECURITY AND SAFETY
====================================================================

Security requirements:

- No hardcoded passwords.
- No SSH private keys in Git.
- No Proxmox API tokens in Git.
- Use Kubernetes Secrets for POC.
- Keep credentials out of logs.
- Use minimal Kubernetes RBAC.
- Use scoped service accounts.
- Use configurable execution timeouts.
- Do not run everything with unrestricted cluster permissions.

Infrastructure safety:

- Default deletion policy: Retain.
- Avoid destructive Terraform changes.
- Prevent simultaneous Terraform state writers.
- Avoid accidental VM recreation.
- Do not delete persistent disks automatically.
- Do not manage the same resource through multiple providers.

Ansible safety:

- Prevent conflicting concurrent Jobs.
- Prevent duplicate deployment of the same revision.
- Preserve pending cleanup operations.
- Do not assume uninstall is supported.
- Never remove unrelated data.
- Apply bounded retries.

Operator safety:

- Recover correctly after restarts.
- Do not duplicate existing Jobs.
- Use finalizers carefully.
- Avoid destructive finalizer behavior.
- Ensure reconciliation remains idempotent.

This is a lab POC, but the architecture should follow sound production engineering principles.


====================================================================
23. REPOSITORY STRUCTURE
====================================================================

Everything must be in one repository.

Suggested layout:

infrastructure-gitops/
|
+-- README.md
+-- Makefile
+-- .gitignore
|
+-- docs/
|   +-- architecture.md
|   +-- installation.md
|   +-- configuration.md
|   +-- reconciliation.md
|   +-- ansible-lifecycle.md
|   +-- troubleshooting.md
|   +-- poc-tests.md
|
+-- operator/
|   +-- cmd/
|   +-- api/
|   |   +-- v1alpha1/
|   +-- internal/
|   |   +-- controller/
|   |   +-- infrastructure/
|   |   +-- ansible/
|   |   +-- scheduler/
|   |   +-- lifecycle/
|   |   +-- execution/
|   +-- config/
|   |   +-- crd/
|   |   +-- rbac/
|   |   +-- manager/
|   |   +-- samples/
|   +-- Dockerfile
|   +-- go.mod
|   +-- go.sum
|
+-- terraform/
|   +-- modules/
|   |   +-- proxmox-vm/
|   |       +-- main.tf
|   |       +-- variables.tf
|   |       +-- outputs.tf
|   |       +-- versions.tf
|   |
|   +-- environments/
|       +-- homelab/
|           +-- main.tf
|           +-- provider.tf
|           +-- variables.tf
|           +-- outputs.tf
|
+-- crossplane/
|   +-- providers/
|   +-- provider-configs/
|   +-- workspaces/
|
+-- ansible/
|   +-- collections/
|   |   +-- ansible_collections/
|   |       +-- homelab/
|   |           +-- web/
|   |           |   +-- galaxy.yml
|   |           |   +-- roles/
|   |           |       +-- nginx/
|   |           |
|   |           +-- system/
|   |               +-- galaxy.yml
|   |               +-- roles/
|   |                   +-- textfile/
|   |
|   +-- playbooks/
|   +-- inventory/
|   +-- execution-environment/
|   +-- runner/
|
+-- examples/
|   +-- virtual-machines/
|   +-- configurations/
|   +-- schedules/
|
+-- argocd/
|   +-- applications/
|   +-- kustomization.yaml
|
+-- deploy/
|   +-- kustomize/
|
+-- tests/
    +-- integration/
    +-- e2e/

You may improve the structure if needed.

Follow normal conventions for Go operators, Terraform, Ansible collections, and Kubernetes manifests.

All project components must remain in the same repository.


====================================================================
24. IMPLEMENTATION PHASES
====================================================================

PHASE 1: REPOSITORY SETUP

- Create repository structure.
- Initialize Go operator project.
- Add documentation.
- Add Terraform module.
- Create both Ansible collections.
- Add container build definitions.
- Add Makefile commands.
- Add Kubernetes manifests.

PHASE 2: CROSSPLANE + OPENTOFU

- Verify provider compatibility.
- Install/configure required Crossplane components.
- Configure persistent Terraform state.
- Configure Proxmox credentials.
- Deploy one VM.
- Verify reconciliation.
- Verify CPU/memory changes.
- Verify state persistence.

PHASE 3: ANSIBLE RUNNER

- Create Ansible Execution Environment.
- Build versioned Ansible collections.
- Create test playbooks.
- Execute Ansible manually through Kubernetes Job.
- Verify Nginx deployment.
- Verify textfile creation.

PHASE 4: CUSTOM OPERATOR

- Implement Configuration CRD.
- Implement infrastructure readiness checks.
- Implement target resolution.
- Implement Kubernetes Job creation.
- Implement Job monitoring.
- Implement revision tracking.
- Implement resource status conditions.

PHASE 5: RECONCILIATION

- Implement OnChange.
- Implement interval scheduling.
- Implement cron scheduling.
- Implement manual execution.
- Implement retry handling.
- Implement concurrency restrictions.
- Implement restart recovery.

PHASE 6: ROLE LIFECYCLE

- Track previously applied roles.
- Detect removed roles.
- Read lifecycle metadata.
- Execute supported uninstall operations.
- Handle unsupported uninstall.
- Preserve pending cleanup state.
- Handle cleanup failures.

PHASE 7: END-TO-END TESTING

- Test initial VM provisioning.
- Test Terraform updates.
- Test both Ansible collections.
- Test collection version bumps.
- Test role removal.
- Test cron scheduling.
- Test interval scheduling.
- Test manual execution.
- Test drift reconciliation.
- Test failure handling.
- Test operator restarts.

PHASE 8: ALTERNATIVE PROVIDER EVALUATION

- Investigate native Crossplane Proxmox provider.
- Compare architecture with OpenTofu.
- Document benefits and limitations.
- If feasible, provide an isolated example.


====================================================================
25. END-TO-END ACCEPTANCE TESTS
====================================================================

TEST 1: VM PROVISIONING

Create one Debian 13 VM through:

Git -> ArgoCD -> Crossplane -> OpenTofu -> Proxmox

Verify:

- VM exists.
- VM boots.
- SSH works.
- Terraform state persists.
- Crossplane reports readiness.


TEST 2: VM RESOURCE UPDATE

Change:

CPU: 2 -> 4
Memory: 2048 -> 4096

Verify:

- Git change is detected.
- OpenTofu reconciliation runs.
- Proxmox configuration is updated.
- VM is not unexpectedly destroyed.
- Persistent disks remain intact.

Document whether a guest reboot is required.


TEST 3: NGINX INSTALLATION

Add:

homelab.web.nginx

Verify:

curl -fsS http://VM_IP/

Expected:

Web is working!


TEST 4: TEXTFILE CREATION

Add:

homelab.system.textfile

Verify:

cat /opt/gitops-demo/hello.txt

Expected:

Hello from Infrastructure GitOps!


TEST 5: NGINX COLLECTION UPGRADE

Update Nginx collection:

1.0.0 -> 1.0.1

Change page content.

Verify:

- Ansible Job starts.
- New content is deployed.
- AppliedRevision updates.
- HTTP response changes.


TEST 6: PINNED VERSION BEHAVIOR

Modify the source code of an Ansible role.

Do not bump the declared collection version.

Verify:

- No immediate deployment occurs.
- Scheduled execution still uses the previously pinned collection artifact.


TEST 7: NGINX REMOVAL

Remove:

homelab.web.nginx

Keep:

uninstallRemovedRoles: true

Verify:

- Operator detects removed role.
- Metadata reports uninstall support.
- Role executes with nginx_state: absent.
- Nginx is removed.
- HTTP service becomes unavailable.


TEST 8: UNSUPPORTED UNINSTALL

Remove:

homelab.system.textfile

Keep:

uninstallRemovedRoles: true

Verify:

- Operator detects role removal.
- Metadata reports uninstall unsupported.
- Cleanup is skipped.
- File remains on disk.
- Warning is recorded.


TEST 9: CRON SCHEDULING

Configure:

schedule:
  type: cron
  expression: "0 8 * * *"
  timezone: Europe/Vilnius

Verify scheduling behavior.

For testing, temporarily use a frequent schedule.

Do not wait until the next day just to validate the mechanism.


TEST 10: INTERVAL SCHEDULING

Configure:

schedule:
  type: interval
  duration: 5m

Verify periodic Ansible execution.


TEST 11: MANUAL RECONCILIATION

Trigger a manual run using Kubernetes annotation.

Verify:

- A new Job starts.
- It uses pinned collection versions.
- Execution status updates.


TEST 12: ANSIBLE FAILURE

Introduce an intentionally failing task.

Verify:

- Job failure is detected.
- AppliedRevision does not advance.
- Retry policy works.
- Error is visible in resource status.


TEST 13: OPERATOR RESTART

Restart the operator while an Ansible Job is running.

Verify:

- Operator discovers existing Job.
- Duplicate Jobs are not created.
- Execution status is recovered correctly.


TEST 14: CONFIGURATION DRIFT

Deploy the textfile role.

Manually modify:

/opt/gitops-demo/hello.txt

Trigger manual or scheduled reconciliation.

Verify:

- Ansible restores the desired file content.


TEST 15: TERRAFORM DRIFT

Manually modify the VM CPU in Proxmox.

Verify:

- OpenTofu reconciliation detects drift.
- The configured policy determines whether it is corrected.
- Result is observable in Crossplane/Kubernetes.


====================================================================
26. DEFINITION OF DONE
====================================================================

The POC is complete only when:

[ ] ArgoCD manages the required Kubernetes declarations.

[ ] Crossplane successfully provisions a Proxmox VM.

[ ] OpenTofu uses a persistent state backend.

[ ] Terraform CPU/memory updates work.

[ ] The custom Go operator is deployed.

[ ] Ansible runs inside Kubernetes Jobs.

[ ] Nginx collection successfully installs Nginx.

[ ] curl returns "Web is working!"

[ ] Textfile collection creates the expected file.

[ ] Collection version bump triggers Ansible.

[ ] Removing Nginx triggers supported uninstall.

[ ] Removing textfile role preserves the file.

[ ] Cron scheduling works.

[ ] Interval scheduling works.

[ ] Manual reconciliation works.

[ ] Failed Jobs are handled correctly.

[ ] Operator restarts do not create duplicate Jobs.

[ ] Configuration status reflects execution results.

[ ] Role cleanup state is tracked.

[ ] All components are stored in one Git repository.

[ ] Complete installation documentation exists.

[ ] End-to-end test results are documented.

Do not mark a feature as complete merely because the implementation files exist.

The feature must be tested.

If a test cannot be executed due to missing environmental dependencies, clearly document the limitation.


====================================================================
27. FUTURE IMPROVEMENTS
====================================================================

These features are explicitly NOT required in the initial POC.

However, the architecture should allow them later.

- NetBox dynamic inventory
- NetBox tag-based target selection
- Multiple VM targets per Configuration
- Host group configuration
- Automatic latest collection tracking
- Role-level Git change detection
- Role dependency graph
- Canary deployments
- Rolling deployments
- Vault integration
- Proxmox host configuration
- Bare-metal provisioning
- Infrastructure adoption and migration
- Crossplane native Proxmox provider backend
- Multi-cluster support
- Advanced drift detection
- Approval policies
- CLI tool
- Web interface

Do not overengineer these features now.


====================================================================
28. DOCUMENTATION REQUIREMENTS
====================================================================

Provide complete Markdown documentation.

Required topics:

1. Architecture overview
2. Component responsibilities
3. Repository layout
4. Installation requirements
5. Crossplane setup
6. OpenTofu provider configuration
7. Terraform state backend
8. Proxmox credentials
9. Operator installation
10. Ansible Execution Environment
11. Ansible collection build/versioning
12. VM provisioning
13. Ansible role deployment
14. Collection version upgrades
15. Role removal
16. Lifecycle metadata
17. Cron scheduling
18. Interval scheduling
19. Manual execution
20. Troubleshooting
21. Resource status and logs
22. Security considerations
23. Known limitations
24. End-to-end tests
25. Native Crossplane provider comparison

Include practical commands.

The README should explain how another engineer can reproduce the POC.


====================================================================
29. IMPORTANT IMPLEMENTATION INSTRUCTIONS
====================================================================

Before writing significant code:

1. Inspect the current repository.
2. Inspect the provided Kubernetes and Proxmox environment.
3. Check available access and credentials.
4. Verify current component versions.
5. Verify Crossplane provider compatibility.
6. Verify actual provider CRD schemas.
7. Identify architecture risks.
8. Establish the implementation order.

Use official, current documentation.

Do not blindly copy old Kubernetes, Crossplane or Terraform examples.

Use production-quality engineering practices:

- Idiomatic Go
- Kubebuilder/controller-runtime
- Structured logging
- Proper error handling
- Context-aware operations
- Idempotent reconciliation
- Minimal RBAC
- No hardcoded credentials
- Unit tests
- Integration tests
- Reproducible container builds

Prefer existing Kubernetes features over custom implementations.

Do not introduce unnecessary infrastructure dependencies.

Do not introduce Flux.

Do not implement a custom Terraform state engine.

Do not create a custom frontend.

Do not replace Ansible Runner with a homemade execution engine.

The POC should be simple enough to understand and maintain.

However, it must correctly implement reconciliation and lifecycle management.


====================================================================
30. FINAL EXPECTED RESULT
====================================================================

The final working flow should be:

Declare Infrastructure
        |
        v
Git Commit / Push
        |
        v
ArgoCD Synchronization
        |
        v
Crossplane Reconciliation
        |
        v
OpenTofu / Terraform
        |
        v
Proxmox VM Provisioned
        |
        v
Infrastructure Ready
        |
        v
Custom Operator
        |
        v
Ansible Kubernetes Job
        |
        v
Ansible Runner
        |
        v
Nginx + Textfile Installed
        |
        v
Configuration Ready
        |
        v
Change Collection Version
        |
        v
Ansible Reconciliation
        |
        v
Updated Configuration
        |
        v
Remove Nginx Role
        |
        v
Automatic Uninstall
        |
        v
Nginx Removed
        |
        v
Remaining Configuration Healthy


The long-term goal is a reusable Kubernetes-native Infrastructure GitOps Controller capable of orchestrating Terraform/OpenTofu and Ansible.

Proxmox is the first infrastructure provider.

The implementation must demonstrate that infrastructure provisioning and configuration management can be controlled through one coherent GitOps workflow.

Build the simplest reliable implementation that satisfies these requirements.

Prioritize correctness, maintainability, safe reconciliation, and reproducible testing over unnecessary complexity.

IMPORTANT FINAL INSTRUCTION:

Implement the POC incrementally.

Validate each component before integrating the next one.

Do not silently skip requirements.

Clearly distinguish:

- Implemented functionality
- Tested functionality
- Known limitations
- Future improvements

The final result must be a working POC, not just documentation or example manifests.