# Installation and credentials

## Prerequisites

- Existing MicroK8s/Kubernetes and Argo CD.
- `kubectl`, Go 1.25+, Terraform/OpenTofu, Ansible Core/Galaxy, and Docker for
  local build/test work.
- A dedicated, least-privilege Proxmox API token and SSH public key.
- GitHub Actions enabled for this repository with `GITHUB_TOKEN` package-write
  permission to publish the operator and Runner images to GHCR.
- A Kubernetes docker-registry Secret named `ghcr-pull` in `infrastructure`,
  using a credential with read access to the private GHCR packages.
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

The sample manifests use those image names. Push the `v1.0.1` Git tag to run
[the publish workflow](../.github/workflows/publish-images.yaml), which builds
and publishes both `linux/amd64` images as `1.0.1` using the repository's
`GITHUB_TOKEN`. Actions must be enabled and permit `packages: write`. A private
GHCR package is pulled using `ghcr-pull` by the controller and its Runner Jobs.
Before either workload starts, create `ghcr-pull` in the `infrastructure`
namespace as a `kubernetes.io/dockerconfigjson` Secret using a GitHub
credential with read access to both packages. Use a trusted secret manager or
cluster UI; never commit the registry token or include it in command history.

## Proxmox credentials and state

Create an API token for a dedicated Proxmox identity with permissions limited
to the POC VMID range, template cloning/read, VM configuration/power actions,
storage allocation, and guest-agent reads. Do not use root or place the token
in a `.tfvars` file checked into Git.

The Workspace passes its token from a Kubernetes Secret to OpenTofu using a
Secret key reference. A ProviderConfig supplies the BPG provider configuration.
See [`../crossplane/README.md`](../crossplane/README.md) for the POC manifests
and backend access requirements.

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

The provider's execution ServiceAccount must be able to read/write/list the
backend Secrets and create/read/update/delete Leases in the state namespace.
Confirm the actual installed ServiceAccount and grant only these namespace
permissions before creating a Workspace. Do not grant cluster-wide state
permissions.

## Deploy a VM

The homelab Terraform root uses VMID 201 and address `10.10.10.21/24` as
example values. Verify they are unused before applying. It clones, never
recreates, template `9000`; Proxmox remains responsible for Cloud-Init.
`prevent_destroy` is a second guard against Terraform destroying the guest.
When ready, add a reviewed namespaced Workspace to `apps/` that references the
module/root in this repository at an immutable Git revision, sets
`managementPolicies` without `Delete`, passes token values from a Secret, and
uses the OpenTofu Kubernetes backend. Do not add a fake credential or apply
until the Proxmox token Secret and provider access Role are configured.

Once the Workspace is `Ready=True`, create the VM's SSH Secret in
`infrastructure` (key `ssh-privatekey`), then review and apply the
[`operator/deploy/configuration-sample.yaml`](../operator/deploy/configuration-sample.yaml)
after substituting the actual VM address. Configuration files are opt-in;
the sample is not automatically applied.

## Operator lifecycle

Argo CD applies the CRD, scoped Role/RoleBinding, and operator Deployment from
`apps/operator`. The image must already be available to the cluster. Generated
Jobs run as UID 1000, drop Linux capabilities, use a read-only root filesystem,
mount only the target SSH Secret, and have a bounded deadline.

Read [Configuration](configuration.md), [scheduling](reconciliation.md),
[Ansible versioning](ansible-lifecycle.md), and [troubleshooting](troubleshooting.md)
before creating real resources.
