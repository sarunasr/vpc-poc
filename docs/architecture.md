# Architecture

```text
Git -> Argo CD -> Datacenter + VirtualMachine CRDs
                         |
                    Go operator
                    /          \
 Crossplane Workspace           Configuration CRD
         |                              |
 provider-opentofu               Ansible Job/Runner
         |                              |
 OpenTofu -> Proxmox                    VM
```

Argo CD remains the GitOps reconciler. The root Application recursively syncs
`apps/`; sync waves apply the CRDs before the controller and VM declarations.
The old statically declared Workspace is excluded from that Application
during migration so the Datacenter controller can adopt it without Argo
overwriting its spec or pruning it.

Each namespaced `Datacenter` controls one Crossplane `Workspace`, one
OpenTofu root module, and one Terraform state. Its controller lists every
non-terminating `VirtualMachine` in the namespace whose `datacenterRef` matches,
merges inherited infrastructure defaults, sorts/builds a complete map keyed
by stable Kubernetes VM names, and writes the map to `spec.forProvider.varmap.vms`.
Optimistic Kubernetes resource versions, conflict retries, and manager leader
election serialize Workspace updates. Equal inputs do not cause updates.

The Workspace's module reference combines `spec.terraform.module.source` with
the pinned `version` as a `ref` query. VM parameter edits do not change this
module revision. The operator records a deterministic hash of the effective
Terraform inputs on the Workspace and in Datacenter status. It waits until
provider-opentofu's `Synced` condition reports an observed generation at
least as new as the Workspace generation, `Ready=True`, and the Workspace
outputs contain each expected VM ID and a valid address. The generated
Configuration also rechecks this state through an uncached API reader before
starting Ansible.

The VirtualMachine controller derives a same-named `Configuration` from each
VM's Ansible declarations, resolved target, effective schedule, and execution
limits. A VM owns its generated Configuration; that Configuration continues
to own its Ansible Jobs and execution ConfigMaps. Argo CD manages only the VM
declaration, never the generated Configuration.

Infrastructure and Ansible revisions are independent. The Terraform input
hash tracks module/provider/credential references and the complete VM map.
The Ansible revision tracks target, role/collection pins, role variables,
lifecycle data, and a separate VM identity revision derived from the module,
VM name/ID, node, and template. It excludes general Workspace
revision/generation, schedule, and execution limits. Therefore, CPU/RAM-only
changes reconcile OpenTofu without unnecessarily triggering an Ansible Job,
while target, Ansible content, or VM recreation-identity changes run the
existing Ansible workflow.

The existing Ansible controller retains interval/cron/manual execution,
retry/concurrency controls, pinned collections, lifecycle cleanup, Job
recovery after restart, and status reporting. Schedules do not upgrade
collections; the desired collection version must exist in the immutable
execution image.

Deleting a VirtualMachine removes its map entry. OpenTofu then destroys that
VM, leaving the Workspace and other map entries in place. An empty Datacenter
map is valid. Deleting a Datacenter garbage-collects its Workspace object, but
the Workspace's management policies omit `Delete`, so the provider is not
asked to destroy its managed infrastructure. Datacenter deletion is not the
normal VM removal workflow.

See [installation and migration](installation.md), [reconciliation](reconciliation.md),
[role lifecycle](ansible-lifecycle.md), and [troubleshooting](troubleshooting.md).
