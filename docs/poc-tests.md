# POC validation record

Status separates static/build checks from infrastructure acceptance tests.

## Verified locally

- MicroK8s `v1.35.6` is reachable using only
  `$HOME/.kube/config-vpc-poc`; node is Ready. Argo CD root Application is
  Synced/Healthy. Crossplane 2.4.2, provider-opentofu 1.1.9, and the
  Configuration controller are running.
- The existing Argo CD public service is unrelated to this POC; no changes to
  it were made.
- `provider-opentofu` upstream README, v1.1.9 Workspace CRD, examples, release,
  and current Crossplane 2.4.2 release/chart were checked. The Workspace uses
  remote state; local provider-pod state is not persistent. Kubernetes backend
  uses Secrets plus Lease locking.
- Proxmox token authentication succeeded with a read-only API version request
  (HTTP 200, version 9.2.21). Read-only inventory confirmed the target node
  contains template VMID `9000` only, with `kvm: 0`; VMID `201` was unused.
  No Proxmox resources were changed.
- `terraform fmt`, module init with backend disabled, and
  `terraform validate` passed with BPG provider `v0.116.0`.
- Both Galaxy collection archives built. Both role test playbooks passed
  Ansible syntax checking. The Ansible execution image was rebuilt for
  `linux/amd64`; it runs as the non-root `runner` user, reports Ansible Core
  `2.18.6`, and successfully syntax-checks a collection role.
- Operator unit tests cover stable/effective hashing, interval missed-run
  behavior, local-time cron behavior, Runner inventory/playbook generation,
  supported Nginx cleanup, and safe skipping for unsupported textfile cleanup.
- `go test ./...` and `go vet ./...` passed. The operator image built for
  `linux/amd64` and its `--help` smoke test passed.
- Operator CRD/RBAC/Deployment passed `kubectl --dry-run=client` validation
  using the specified kubeconfig.
- `make test` and `git diff --check` passed.
- GitHub Actions runs `38079592522` and `38080723705` successfully published
  both `linux/amd64` image versions, `1.0.1` and `1.0.2`. Anonymous GHCR
  bearer-token requests for both `1.0.2` manifests returned HTTP 200. Argo
  rolled out the `1.0.2` operator without an image-pull Secret.

## Not run / blocked

- The configured Proxmox token currently has VM administration privileges on
  protected VMIDs `101` and `102`, VMID `201`, and template `9000`. Narrow its
  ACL before provisioning; no Proxmox resources were changed.
- The installed provider-opentofu system ClusterRole grants wildcard access to
  Secrets, ConfigMaps, and Leases cluster-wide. The Kubernetes state backend
  cannot be used safely until this provider access is restricted to
  `infrastructure`.
- The private Git repository still needs a read-only credential available to
  the provider for a pinned module revision.
- No OpenTofu Workspace or VM was applied. VMIDs `101`/`102`, template `9000`,
  and Proxmox networking/storage were not changed.
- No Debian target VM/SSH Secret exists for end-to-end Ansible install,
  upgrade, uninstall, cron, or drift tests.
- BPG provider does not document a dedicated KVM-enable/disable VM argument.
  The module clones the already `kvm: 0` template and sets CPU type `qemu64`;
  verify the cloned Proxmox config before relying on it.
- Native provider comparison is a source-level evaluation only; it is not
  installed or tested against Proxmox.

Do not claim the final end-to-end acceptance checklist is complete until the
blocked tests are actually executed.
