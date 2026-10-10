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
- GitHub Actions run `38079592522` successfully published both `linux/amd64`
  images as `1.0.1`. An anonymous manifest check confirmed the packages are
  publicly readable, as intended. The `1.0.2` release removes references to a
  private-registry pull Secret.

## Not run / blocked

- No Proxmox API token exists in the supplied infrastructure notes. No API
  token, root login, SSH to Proxmox, or other kubeconfig was used.
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
