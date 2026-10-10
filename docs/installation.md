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

The committed root Argo CD Application watches `apps/`. Pushing changes causes
Argo CD to create the Crossplane Application, install the Crossplane Helm
release at 2.4.2, install provider-opentofu 1.1.9, create the `infrastructure`
namespace, and install the Configuration CRD/operator resources. Crossplane
is installed before a separate Provider child Application syncs
`provider-opentofu`, so Argo discovers the Provider CRD only after Crossplane
has installed it.

## Build images

From the repository root:

```sh
make collections-build
make ansible-image
make operator-image
```

The sample manifests use those image names. Push the `v1.0.2` Git tag to run
[the publish workflow](../.github/workflows/publish-images.yaml), which builds
and publishes both `linux/amd64` images as `1.0.2` using the repository's
`GITHUB_TOKEN`. Actions must be enabled and permit `packages: write`. GHCR
`1.0.2` packages are public and do not require Kubernetes image-pull
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

## Deploy a VM

The homelab Terraform root uses VMID 201 and address `10.10.10.21/24` as
example values. Verify they are unused before applying. It clones, never
recreates, template `9000`; Proxmox remains responsible for Cloud-Init.
`prevent_destroy` is a second guard against Terraform destroying the guest.
The Argo-managed [`homelab Workspace`](../apps/infrastructure/homelab-workspace.yaml)
pins the module to an immutable Git revision, uses the Kubernetes state
backend, and omits `Delete` from its management policies. The target address
has been confirmed reserved, and the Proxmox token cannot manage VMIDs
`101`/`102` or modify template `9000`. It reads credentials from Kubernetes
Secrets; do not add fake credential values. Proxmox also requires the token to
have `SDN.Use` on `/sdn/zones/localnetwork/vmbr0`; until that is granted, the
Workspace remains Observe-only and cannot provision.

The dedicated `infrastructure/homelab-vm-ssh` Secret already contains the
separate VM keypair; do not reuse the GitHub deploy key. After the Workspace
is `Ready=True`, review and apply the
[`operator/deploy/configuration-sample.yaml`](../operator/deploy/configuration-sample.yaml)
after confirming the actual VM address. Configuration files are opt-in; the
sample is not automatically applied.

## Operator lifecycle

Argo CD applies the CRD, scoped Role/RoleBinding, and operator Deployment from
`apps/operator`. The image must already be available to the cluster. Generated
Jobs run as UID 1000, drop Linux capabilities, use a read-only root filesystem,
mount only the target SSH Secret, and have a bounded deadline.

Read [Configuration](configuration.md), [scheduling](reconciliation.md),
[Ansible versioning](ansible-lifecycle.md), and [troubleshooting](troubleshooting.md)
before creating real resources.
