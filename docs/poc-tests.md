# POC validation record

This record distinguishes completed end-to-end checks from remaining work.

## Unified Datacenter/VirtualMachine refactor

The refactor has local automated validation, but it has **not yet been pushed
or applied to the cluster**. No Datacenter or VirtualMachine resources for the
new API have been created in the live cluster as part of this change.

Locally verified:

- `go test ./...` covers deterministic complete-map aggregation, inherited
  defaults, duplicate VMID rejection within a Datacenter, independent VMID
  scopes across Datacenters, pinned module refs, Workspace output matching,
  schedule/infrastructure-only revision behavior, Workspace adoption, complete
  map updates, no-op updates, and empty-map handling.
- `go vet ./...`, `terraform fmt -check -recursive terraform`, backend-disabled
  `terraform init`, and `terraform validate` pass locally.
- Kubernetes server-side dry runs pass for the operator/CRD/RBAC/Deployment
  manifests and the updated root Argo Application using only
  `$HOME/.kube/config-vpc-poc`.
- `git diff --check` passes.
- The live Workspace is still `infrastructure/homelab`, generation 5, and
  remains managed by the current Argo Application. The one-time migration has
  not been executed.

Still to verify after deployment:

- Argo installs CRDs before the operator and VM resources.
- Existing Workspace state migrates from `module.vm` to
  `module.vms["web01"]` without replacing VM 201.
- Datacenter/VM controller status and derived Configuration work end to end.
- VM 202 provisioning/Ansible: this must wait until the Proxmox token ACL
  includes VMID 202 and the restricted jump key permits `10.10.10.22:22`.
- CPU/RAM-only edits avoid unnecessary Ansible; collection/role changes
  trigger only Ansible; VM deletion removes only that VM.
- Empty Datacenter, concurrent declarations, and operator restart recovery
  behave as expected in the live cluster.

## Verified

- MicroK8s is reachable using only `$HOME/.kube/config-vpc-poc`; the node is
  Ready and the Argo CD root Application is Synced/Healthy. Crossplane 2.4.2,
  provider-opentofu 1.1.9, and the Configuration controller are installed.
- Proxmox token authentication succeeded. Effective permissions allow VM
  administration only on VMID `201`, audit/clone on template `9000`, storage
  allocation on `local`, and `SDN.Use` only on
  `/sdn/zones/localnetwork/vmbr0`. VMIDs `101`/`102` and template `9000` were
  not changed.
- The previous Argo-managed Workspace was Ready, with OpenTofu state persisted in the
  Kubernetes backend. VMID `201` is running at `10.10.10.21`, cloned from
  template `9000`; its configuration has `kvm: 0`, CPU `qemu64`, 1 core,
  1 GiB memory, and an 8 GiB disk. No active backend Lease remains.
- The previous Terraform formatting, backend-disabled module initialization,
  and validation passed with BPG provider `v0.116.0`.
- A dedicated restricted Proxmox SSH jump key and separate VM key were used
  from Kubernetes to authenticate to `debian@10.10.10.21`. The successful
  SSH probe ran through the jump host without using the workstation key.
- The first operator-managed `web01` Ansible Job succeeded: both
  `homelab.system.textfile` and `homelab.web.nginx` ran, with 7 OK, 5 changed,
  0 unreachable, and 0 failed tasks.
- The operator's full 64-character `AppliedRevision` tracking was verified.
  The `onChange` setting is enabled and the final scheduled interval is 24h.
- Requesting unavailable collection version `homelab.web:9.9.9` set
  `Ready=False` with reason `CollectionVersionUnavailable` and did not create
  a Job. The supported pinned version `1.0.0` was restored.
- With a temporary 30-second interval, the controller created and completed
  scheduled Job `web01-0fa28220-6c05e5-1` successfully in 28 seconds.
- Removing `homelab.web.nginx` caused the controller's supported cleanup to
  stop and disable Nginx, remove the managed landing page, and uninstall the
  Nginx package. The Ansible run completed with 0 unreachable and 0 failed
  tasks. The normal 24-hour interval and both desired roles were restored.
- Returning to a previously applied revision exposed a stale completed-Job
  name collision. The v1.0.6 fix was published and rolled out; reverting the
  temporary role-order change then created a fresh Job
  (`web01-4577428e-7ec825-1`) rather than reusing the old completed Job. It
  ran successfully with the v1.0.6 Runner, reinstalled/enabled Nginx, and
  reported the full 64-character applied revision with both roles applied.
- The v1.0.6 GitHub Actions image-publish workflow succeeded, Argo CD rolled
  out the v1.0.6 controller, and the final `web01` Configuration is Synced
  with `onChange: true`, both roles, and its 24-hour schedule. No active
  Ansible Job remains.
- Before this refactor, `go test ./...`, `go vet ./...`, `make test`, and
  `git diff --check` passed. The previous Configuration CRD, RBAC, Deployment,
  and sample Configuration passed Kubernetes dry-run validation.

## Remaining validation

- A valid collection upgrade between two published collection versions and
  deliberate guest drift correction have not been tested.
- Destructive VM deletion was previously disabled. This refactor intentionally
  enables deletion through removal from the complete Terraform VM map; it has
  not yet been tested against the live cluster.
- provider-opentofu's installed system ClusterRole grants wildcard access to
  Secrets, ConfigMaps, and Leases cluster-wide. This is explicitly accepted
  for this personal homelab POC and remains a security limitation that must
  be addressed before production.

Do not treat manifest presence or a Ready status alone as proof that an
end-to-end test passed.
