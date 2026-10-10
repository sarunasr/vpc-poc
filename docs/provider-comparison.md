# Infrastructure provider comparison

| Topic | Crossplane + provider-opentofu + BPG | Native `provider-proxmox-bpg` |
|---|---|---|
| Abstraction | Crossplane Workspace runs OpenTofu HCL | Crossplane managed Proxmox resources |
| Scope | Arbitrary Terraform root modules and providers | Proxmox resources exposed by the provider |
| State | Required; use Kubernetes Secret/Lease remote backend | Managed-resource identity/status rather than shared Terraform state |
| Resource granularity | One Workspace may manage multiple Terraform resources/providers | Individual Crossplane managed resources |
| Drift | OpenTofu plan/apply polling; provider docs state 10-minute default | Upjet observation/reconciliation; field support depends on generated resource |
| Deletion safety | Backend and module lifecycle; configure Workspace policy and `prevent_destroy` | Crossplane deletion policy/management policies per resource |
| Recovery | Restore state Secret/Lease and reconcile actual infrastructure | Recreate/adopt individual managed-resource state/identity |
| Maturity evidence | Upbound maintained provider, v1.1.9 observed 2026-09-27; explicit known timeout/state limitations | Community provider generated from BPG; latest observed release v1.21.0 on 2026-10-09; small maintainer/release footprint |
| Operational cost | OpenTofu runner, provider config, state backup, Git module auth | Additional provider package/CRDs and Proxmox credential config |

The main POC uses OpenTofu because it matches existing HCL and can manage
Proxmox, NetBox, DNS and other providers in one root module/state. The provider
does not persist local state, so the Kubernetes backend is mandatory. It warns
that terminating a long operation at the provider timeout can lose state and
leave a stale lock.

The native alternative [`valkiriaaquatica/provider-proxmox-bpg`](https://github.com/valkiriaaquatica/provider-proxmox-bpg)
is actively release-automated from the BPG Terraform provider and documents
Crossplane v2 compatibility, but has a small contributor/release footprint
compared with the upstream BPG Terraform provider. Generated examples do not
prove adoption, resource import, deletion safety, or complete Proxmox field
coverage. It remains an isolated evaluation only.

Do not install both providers to manage the same VM. Compare clone/update/
adoption behavior and the current no-nested-KVM requirement on a disposable
resource before considering the native provider for the main path.
