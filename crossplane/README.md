# Crossplane and OpenTofu

## Verified component facts

- Crossplane `v2.4.2` is the current stable release observed during POC
  development; the matching `crossplane` Helm chart is published at
  `charts.crossplane.io/stable`.
- `upbound/provider-opentofu:v1.1.9` was the latest release observed. The
  upstream repository had recent commits in September 2026 and documents a
  Crossplane Workspace managed resource. Crossplane v2 namespaced APIs use
  `opentofu.m.upbound.io/v1beta1`.
- The Workspace CRD schema supports `source: Inline|Remote`, module content or
  a remote module source, vars/varmap/varFiles, env Secret references,
  `remotePullPolicy`, provider config references and management policies.
- The provider runs OpenTofu; it does not implement a second Terraform engine.
  It polls existing workspaces every 10 minutes by default (configurable) and
  reconciles spec changes immediately.

Sources: [provider README](https://github.com/upbound/provider-opentofu),
[Workspace CRD at v1.1.9 source revision](https://github.com/upbound/provider-opentofu/blob/75ce45393cbd9d0c537a327afaaf3e04bc87cc53/package/crds/opentofu.upbound.io_workspaces.yaml),
[OpenTofu Kubernetes backend](https://opentofu.org/docs/language/settings/backends/kubernetes/),
[Crossplane releases](https://github.com/crossplane/crossplane/releases).

## State and failure behavior

Provider-opentofu explicitly does **not** persist local state in its pod. Every
module must configure a remote backend. The Kubernetes backend stores the
state in Secrets and obtains a Lease for locking; a lost pod does not lose the
state Secret. The 1 MiB Kubernetes Secret limit applies.

The provider README warns that exceeding its `--timeout` (default 20 minutes)
kills the OpenTofu process, which can lose state and leak external resources;
the lock may require manual recovery. It also documents that it emits no
Workspace event until after apply succeeds. Poll intervals and timeouts must
therefore be tuned and failures checked at both Kubernetes and Proxmox layers.
Provider code performs periodic `tofu plan` and applies detected drift; verify
each BPG-managed field in the POC before relying on drift correction.

State recovery: stop/suspend new Workspace reconciles, inspect current
Proxmox resources and the corresponding `tfstate-<workspace>-<suffix>` Secret,
restore a verified state backup if needed, and only remove an expired Lease
after proving that no OpenTofu process is still applying. Never delete the
state Secret as a quick fix.

## Required setup before a Workspace

1. Create a dedicated Proxmox API user/token; grant only the permissions
   needed for the declared resources and VMID range.
2. Store the token in a Kubernetes Secret in `infrastructure`. Supply it as a
   Workspace environment Secret reference; never render it into a manifest.
3. The Terraform root's BPG provider block sets the endpoint and TLS
   verification; the Workspace injects `api_token` from the Kubernetes
   Secret. Insecure TLS is only for this self-signed lab server.
4. For this personal homelab POC, the installed provider-opentofu v1.1.9
   system ClusterRole's wildcard access to Secrets, ConfigMaps and Leases is
   explicitly accepted. The Kubernetes state data still remains in
   `infrastructure` under its dedicated backend suffix. This broad provider
   permission is a known security limitation that must be addressed before
   production; do not mistake it for namespace-scoped access.
5. Configure `backend "kubernetes"` with `namespace = "infrastructure"`,
   `secret_suffix = "homelab-proxmox"`, and `in_cluster_config = true`.
6. Set Workspace `managementPolicies` to omit `Delete`. Do not destroy the
   Workspace or state as a way of deleting a VM.

The exact provider ServiceAccount name is assigned at package install and
must be checked in the target cluster. Crossplane 2.4.2 adds the broad system
permissions unconditionally to each provider. This POC explicitly accepts
that limitation to keep the existing Argo CD, Crossplane and OpenTofu
architecture; an external state backend would not remove the provider's
unconditional Kubernetes permissions. Before production, replace or constrain
this permission model through a supported and verified mechanism.

The Proxmox token Secret has been configured and authenticated successfully
with a read-only API request. Its current ACL is broader than the POC requires:
it grants VM administration on VMIDs 101, 102, 201, and template 9000. Narrow
the token before provisioning so 101/102 and the template cannot be modified.
No Workspace or VM has been applied.

## Module sourcing

Provider-opentofu downloads remote modules with `tofu init -from-module`.
Argo CD's existing read-only GitHub deploy key is copied into the
`crossplane-system/provider-opentofu-git-auth` Secret, together with the
`argocd-ssh-known-hosts-cm` host keys. The
`provider-opentofu-git` DeploymentRuntimeConfig uses an init container to
install the Alpine OpenSSH client into an ephemeral volume and stage the key
with SSH-safe ownership and mode. The provider mounts the staged data
read-only and uses `GIT_SSH_COMMAND`; the private key is never placed in Git.
Refresh the Secret if the repository deploy key or GitHub host keys are
rotated.

Workspaces must fetch `terraform/environments/homelab` at an immutable commit
SHA, not a moving branch ref. When upgrading provider-opentofu, update the
runtime configuration's Deployment selector to match the new generated
provider revision before rolling it out.

## BPG template limitations

The reusable module uses the existing full-clone template `9000`, storage
`local`, bridge `vmbr0`, and QEMU `qemu64`. The template's `kvm: 0` setting is
inherited on clone; current BPG VM resource docs expose CPU type and generic
QEMU arguments, but no dedicated `kvm` boolean. Validate the actual cloned VM
configuration before any production use. Avoid applying if the provider
changes the template's inherited `kvm: 0` configuration.
