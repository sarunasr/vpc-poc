# POC validation record

Status separates static/build checks from infrastructure acceptance tests.

## Verified locally

- MicroK8s `v1.35.6` is reachable using only
  `$HOME/.kube/config-vpc-poc`; node is Ready. Argo CD root Application is
  Synced/Healthy. Crossplane 2.4.2, provider-opentofu 1.1.9, and the
  Configuration controller are running. The provider Git runtime rollout is
  Synced/Healthy.
- The existing Argo CD public service is unrelated to this POC; no changes to
  it were made.
- `provider-opentofu` upstream README, v1.1.9 Workspace CRD, examples, release,
  and current Crossplane 2.4.2 release/chart were checked. The Workspace uses
  remote state; local provider-pod state is not persistent. Kubernetes backend
  uses Secrets plus Lease locking.
- Proxmox token authentication succeeded with a read-only API version request
  (HTTP 200, version 9.2.21). The effective ACL grants VM administration only
  on VMID `201`, no privileges on `101`/`102`, audit/clone on template `9000`,
  storage allocation on `local`, and `SDN.Use` only on
  `/sdn/zones/localnetwork/vmbr0`.
- The Argo-managed Workspace is Ready. OpenTofu state is persisted in
  `tfstate-homelab-homelab-proxmox`; no active Lease holder remains. VMID `201`
  is running at `10.10.10.21`, cloned from template `9000`; its Proxmox config
  confirms `kvm: 0`, CPU `qemu64`, 1 core, 1 GiB memory, and an 8 GiB disk.
  VMIDs `101`/`102` and template `9000` were not changed.
- `terraform fmt`, module init with backend disabled, and
  `terraform validate` passed with BPG provider `v0.116.0`.
- Both Galaxy collection archives built. Both role test playbooks passed
  Ansible syntax checking. The Ansible execution image was rebuilt for
  `linux/amd64`; it runs as the non-root `runner` user, reports Ansible Core
  `2.18.6`, and successfully syntax-checks a collection role.
- Operator unit tests cover stable/effective hashing, interval missed-run
  behavior, local-time cron behavior, Runner inventory/playbook generation,
  separate and safely staged ProxyJump keys, unsafe jump setting rejection,
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

- The installed provider-opentofu system ClusterRole grants wildcard access to
  Secrets, ConfigMaps and Leases cluster-wide. This is explicitly accepted for
  this personal homelab POC; it is a known security limitation that must be
  addressed before production. Crossplane v2.4.2 adds these permissions
  unconditionally, and an external state backend does not remove them.
- The provider Git SSH Secret has been populated from Argo CD's read-only
  deploy key and public GitHub known-host data. The provider pod reports the
  key as mode 0600, and a private-repository `git ls-remote` succeeded.
- A dedicated Ed25519 VM/Ansible keypair is stored in the
  `infrastructure/homelab-vm-ssh` Secret; private key material was not
  displayed or committed.
- The guest agent confirms the VM's `10.10.10.21` address, but Kubernetes pods
  cannot connect to guest SSH port 22; the Proxmox host's SSH port 22 is also
  unreachable from the cluster. The VM subnet is private/NATed behind
  `vmbr0`. The Configuration sample supports a dedicated jump host, but the
  Proxmox SSH firewall and restricted jump key must be configured before
  applying it. No Configuration or Ansible Job has been applied.
- Ansible install/upgrade, scheduling, collection versioning, role removal,
  uninstall, and drift tests remain unrun until that network path is available.
- BPG provider does not document a dedicated KVM-enable/disable VM argument.
  The actual clone retained `kvm: 0` and CPU type `qemu64`; verify this
  remains true after changes to the template or module.

Do not claim the final end-to-end acceptance checklist is complete until the
blocked tests are actually executed.
