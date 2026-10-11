# Infrastructure GitOps POC

This repository manages Proxmox VMs and their Ansible configuration through
Argo CD, Crossplane `provider-opentofu`, OpenTofu, and the existing Go operator
and Ansible Runner Jobs. One `Datacenter` owns one Crossplane Workspace and
Terraform state; its `VirtualMachine` resources are aggregated into a complete
Terraform `vms` map.

## Add a VM

Create the datacenter once in
`apps/datacenters/dc1/datacenter.yaml`, pinning a module release and keeping
credentials in Kubernetes Secrets:

```yaml
apiVersion: infra.example.io/v1alpha1
kind: Datacenter
metadata:
  name: dc1
  namespace: infrastructure
spec:
  terraform:
    workspaceName: homelab
    module:
      source: git::ssh://git@github.com/sarunasr/vpc-poc.git
      version: v1.0.0
      entrypoint: terraform/environments/homelab
    providerConfigRef:
      kind: ProviderConfig
      name: default
    proxmoxAPIToken:
      name: proxmox-credentials
      key: api-token
    sshPublicKeys:
      name: homelab-vm-ssh
      key: ssh-public-keys
  proxmox:
    defaultNode: srv2050453
    defaultTemplate: 9000
  vmDefaults:
    cores: 1
    memory_mb: 1024
    disk_size_gb: 8
    bridge: vmbr0
    gateway: 10.10.10.1
    dns_servers: [1.1.1.1]
    cloud_init_user: debian
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
    execution:
      maxConcurrent: 2
      timeout: 30m
      maxRetries: 3
```

Then create one file under `apps/datacenters/dc1/vms/` for each VM. For
example, `web02.yaml`:

```yaml
apiVersion: infra.example.io/v1alpha1
kind: VirtualMachine
metadata:
  name: web02
  namespace: infrastructure
spec:
  datacenterRef: dc1
  vm:
    name: gitops-poc-202
    vm_id: 202
    ipv4_cidr: 10.10.10.22/24
  ansible:
    collections:
      - name: homelab.web
        version: 1.0.0
    roles:
      - name: homelab.web.nginx
    lifecycle:
      uninstallRemovedRoles: true
```

Then **commit and push**. Argo CD syncs the declaration; the operator updates
the datacenter Workspace with every declared VM, waits for the current
Workspace generation and matching VM outputs, and derives the Ansible
`Configuration` and Job. No Terraform edit, Workspace Git SHA update, or
separate `Configuration` file is needed.

The shared defaults and pinned Terraform module are in
[`apps/datacenters/dc1/datacenter.yaml`](apps/datacenters/dc1/datacenter.yaml).
VM 201's existing declaration is in
[`web01.yaml`](apps/datacenters/dc1/vms/web01.yaml); the second-VM example
is in [`web02.yaml`](apps/datacenters/dc1/vms/web02.yaml).
The Kubernetes resources are under `apps/` because the existing root
Application watches that directory recursively.

## How reconciliation works

- `Datacenter` specifies the Workspace name, immutable Terraform module
  revision, ProviderConfig, Secret key references, Proxmox defaults, and
  shared Ansible settings. Secret values stay in Kubernetes Secrets.
- `VirtualMachine.spec.vm` uses the existing Terraform module's input names:
  `vm_id`, `node_name`, `template_id`, `cores`, `memory_mb`, `disk_size_gb`,
  `bridge`, `ipv4_cidr`, `gateway`, `dns_servers`, and `cloud_init_user`.
  Missing values inherit from `Datacenter.spec.vmDefaults` or `spec.proxmox`;
  explicit VM values win. The resource name is the stable Terraform map key.
- All non-terminating VMs whose `datacenterRef` matches are placed in one
  deterministic `varmap.vms` map. Removing a VM removes only that map entry;
  OpenTofu then destroys only that VM. An empty map is passed when the last VM
  is removed, leaving the Workspace and its state intact.
- Ansible target user, SSH Secret, ports, and ProxyJump inherit from the
  Datacenter defaults and can be overridden in `spec.target`. Schedule,
  execution limits, and `onChange` similarly inherit and may be overridden.
  Collections and roles are declared on each VM; the operator owns a
  same-named `Configuration` and runs the existing Ansible Runner workflow.
- CPU, memory, disk, or network edits change Workspace inputs without changing
  the Ansible content revision. Changes to collections, roles, variables,
  target, role lifecycle, or VM recreation identity (module, VM name/ID, node,
  or template) do change the Ansible revision. Schedule-only and execution-limit
  changes update behavior without needlessly changing that content revision.
- Terraform modules and Ansible collections remain pinned. Changing VM values
  does not require changing the module revision. Update
  `spec.terraform.module.version` only to upgrade the Terraform module.

For a VM change, edit its YAML. To adjust CPU/RAM, set `vm.cores` or
`vm.memory_mb`. Add/remove entries under `ansible.roles` to reconcile roles;
`uninstallRemovedRoles: true` runs supported uninstall tasks and reports a
warning for roles without uninstall support. Change a collection's pinned
version to request that version; it must already be present in the operator's
Ansible execution image. `reconciliation.schedule` accepts `interval` with a
Go duration such as `24h`, or `cron` with an expression and optional IANA
timezone. Delete the VM YAML to destroy that VM (POC behavior).

## Architecture and operations

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

The root Argo Application installs the CRDs before the controller and VM
declarations. The operator Deployment and RBAC are under `apps/operator`;
operator-generated Configurations are not Argo-managed. The old statically
managed Workspace is excluded from the root Application and adopted by its
Datacenter. See [`docs/installation.md`](docs/installation.md) for the
one-time migration and credential requirements.

Useful status and diagnostics:

```sh
kubectl -n infrastructure get datacenters,virtualmachines,configurations
kubectl -n infrastructure describe virtualmachine web02
kubectl -n infrastructure get workspaces.opentofu.m.upbound.io homelab -o yaml
kubectl -n infrastructure get jobs
kubectl -n infrastructure logs job/<configuration-job>
```
Use only the designated kubeconfig, `$HOME/.kube/config-vpc-poc`, for this
homelab.

See [`docs/architecture.md`](docs/architecture.md),
[`docs/ansible-lifecycle.md`](docs/ansible-lifecycle.md),
[`docs/reconciliation.md`](docs/reconciliation.md),
[`docs/troubleshooting.md`](docs/troubleshooting.md), and
[`docs/poc-tests.md`](docs/poc-tests.md).

## Build and test

Required local tools: Go 1.25+, Terraform/OpenTofu, Ansible Core/Galaxy, and
Docker.

```sh
make test
make terraform-validate
make ansible-image
make operator-image
```

## POC limitations

- VM declaration deletion causes automatic OpenTofu destruction. Do not use
  this behavior for production workloads.
- provider-opentofu's existing cluster-wide wildcard access to Kubernetes
  Secrets, ConfigMaps, and Leases is explicitly accepted for this personal
  homelab POC. It is a known security limitation to address before production.
- The private Git repository and Kubernetes Secret/state handling remain
  operational dependencies. Never put credentials or private SSH keys in Git.

The end-to-end test record, including what has and has not been run, is in
[`docs/poc-tests.md`](docs/poc-tests.md).
