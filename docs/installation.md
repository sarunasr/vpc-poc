# Installation and credentials

## Prerequisites

- Existing MicroK8s/Kubernetes and Argo CD.
- `kubectl`, Go 1.25+, Terraform/OpenTofu, Ansible Core/Galaxy, and Docker for
  local build/test work.
- A dedicated, least-privilege Proxmox API token and SSH public key.
- GitHub Actions enabled for this repository with `GITHUB_TOKEN` package-write
  permission to publish the operator and Runner images to GHCR.
- The existing Proxmox template `9000`, node `srv2050453`, storage `local`,
  bridge `vmbr0`, and a free static guest address.

Never copy the kubeconfig, Proxmox API token, or private SSH key into Git. All
cluster commands must specify the intended kubeconfig explicitly:

```sh
KUBECONFIG="$HOME/.kube/config-vpc-poc"
kubectl --kubeconfig "$KUBECONFIG" get nodes
```

The root Argo CD Application watches `apps/`. It installs the infrastructure
namespace, Crossplane/provider child Applications, operator CRDs, RBAC, and
controller Deployment. Sync waves apply the CRDs before the controller and
VirtualMachine declarations.

## Build images

From the repository root:

```sh
make collections-build
make ansible-image
make operator-image
```

The sample manifests use those image names. Push the `v1.0.8` Git tag to run
[the publish workflow](../.github/workflows/publish-images.yaml), which builds
and publishes both `linux/amd64` images as `1.0.8` using the repository's
`GITHUB_TOKEN`. Actions must be enabled and permit `packages: write`. GHCR
`1.0.8` packages are public and do not require Kubernetes image-pull
credentials. GHCR responds to unauthenticated manifest requests with a bearer
challenge; clients request an anonymous pull token and then fetch the image.
The Git repository remains private, but public images expose the published
binaries and embedded Ansible collections to anyone who can pull them. Do not
include secrets in either image. If a future package is created as private,
change its visibility to Public in GitHub Packages.

## Proxmox credentials and state

Create an API token for a dedicated Proxmox identity with permissions limited
to the POC VMID range, template cloning/read, VM configuration/power actions,
storage allocation, and guest-agent reads. Do not use root or place the token
in a `.tfvars` file checked into Git.

The Workspace passes its token from a Kubernetes Secret to OpenTofu using a
Secret key reference. The Terraform root configures the BPG provider; its
namespaced ProviderConfig selects OpenTofu execution settings. See
[`../crossplane/README.md`](../crossplane/README.md) for the POC manifests and
backend access requirements.

The selected OpenTofu Kubernetes backend stores state in a Secret named from
the OpenTofu workspace and `secret_suffix`; it uses Kubernetes Lease objects
for state locking. Configure a dedicated namespace/suffix per independent root
module. Back up both Secrets (state) and Leases (locks) in the
`infrastructure` namespace; state recovery is restoring the matching state
Secret and removing a stale Lease only after verifying no apply is active.
Provider-opentofu itself does not persist its local `/tf` directory. Its task
timeout can terminate a running OpenTofu operation without a clean state
write, so choose a timeout longer than expected plans/applies and inspect
Proxmox and state before retrying such a failure.

The provider's execution ServiceAccount needs access to the backend Secrets
and Leases. For this personal homelab POC, the existing provider-opentofu
system ClusterRole's wildcard access to Secrets, ConfigMaps and Leases across
the cluster is explicitly accepted. State is still configured in the
`infrastructure` namespace with a dedicated suffix. This is a known security
limitation and must be addressed before production.

## Private Terraform module access

The provider runtime uses a read-only copy of Argo CD's GitHub SSH deploy key
and the existing GitHub known-host data. The key is stored only in the
`crossplane-system/provider-opentofu-git-auth` Secret. Its
`DeploymentRuntimeConfig` init container stages the key with SSH-safe
ownership/mode, installs the OpenSSH client into an ephemeral volume, and adds
the provider's UID 2000 to pod-local passwd/group files; the provider mounts
the resulting data read-only. The key is not committed to this repository.
Refresh the Secret if the deploy key or known-host data changes. Workspace
module sources must pin an immutable Git commit SHA.

## Migrate the existing Workspace

The existing `infrastructure/homelab` Workspace contains the live VM 201 state
and is currently tracked by the root Argo Application. Do not delete it or
create a second Workspace. Before syncing the new Datacenter declaration,
hand ownership to the operator:

1. Mark the Workspace non-prunable:

   ```sh
   kubectl --kubeconfig "$HOME/.kube/config-vpc-poc" -n infrastructure \
     annotate workspace.opentofu.m.upbound.io homelab \
     argocd.argoproj.io/sync-options=Prune=false --overwrite
   ```

2. Apply the updated root Application, which excludes the old static manifest,
   verify the `exclude` field is present on `Application/root`, then remove
   the old resource tracking ID:

   ```sh
   kubectl --kubeconfig "$HOME/.kube/config-vpc-poc" apply \
     -f bootstrap/argocd/root-application.yaml
   kubectl --kubeconfig "$HOME/.kube/config-vpc-poc" -n infrastructure \
     annotate workspace.opentofu.m.upbound.io homelab \
     argocd.argoproj.io/tracking-id-
   ```

3. Sync the root Application. The `dc1` Datacenter controller adopts the same
   Workspace and adds the complete VM map. Terraform's `moved` block maps
   `module.vm` to `module.vms["web01"]`; inspect the plan/state and confirm
   VM 201 remains unchanged before allowing OpenTofu to apply.

The Workspace keeps its existing name and Kubernetes backend state. Its old
manifest remains in Git for reference but is excluded from Argo's recursive
source.

## Declare and add VMs

The Datacenter declaration at
[`../apps/datacenters/dc1/datacenter.yaml`](../apps/datacenters/dc1/datacenter.yaml)
holds the module revision, Secret references, infrastructure defaults, and
Ansible defaults. Each
[`VirtualMachine`](../apps/datacenters/dc1/vms/web01.yaml) file contains both
Terraform-compatible VM inputs and pinned Ansible collections/roles. To add a
VM, create one YAML file under `apps/datacenters/dc1/vms/`, then commit and
push. Argo syncs it and the operator aggregates all VMs into the existing
Workspace; no Terraform edits, Workspace SHA updates, or standalone
Configuration manifests are needed.

VM 201 uses a dedicated VM SSH Secret and a separate restricted Proxmox
jump-host Secret. Never reuse the GitHub deploy key. Before provisioning VM
202, grant the Proxmox API token the required VMID 202 permissions and update
the jump key's `permitopen` entry to allow `10.10.10.22:22`. Confirm the
address is reserved and SSH/firewall settings are ready. Removing a VM YAML
automatically destroys that VM through Terraform; this behavior is intentional
for the POC.

## Operator lifecycle

Argo CD applies the Configuration, Datacenter, and VirtualMachine CRDs before
the controller resources in `apps/operator`. The namespaced Role manages the
operator CRDs, generated Jobs and ConfigMaps, leader-election Lease, and
Crossplane Workspace in `infrastructure`. Generated Jobs run as UID 1000,
drop Linux capabilities, use a read-only root filesystem, stage the target and
optional jump SSH keys with mode `0600` into an ephemeral volume, store
accepted host keys in ephemeral `/tmp/known_hosts`, and have a bounded deadline.

Read [Configuration](configuration.md), [scheduling](reconciliation.md),
[Ansible versioning](ansible-lifecycle.md), and [troubleshooting](troubleshooting.md)
before creating real resources.
