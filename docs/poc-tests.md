# POC validation record

This record distinguishes completed end-to-end checks from remaining work.

## Unified Datacenter/VirtualMachine refactor

The refactor is committed as `720ed2558aed0115ae8c4c9a7f4715027f6fd803`,
tagged `v1.0.7`, with the initial `dc1` and `web01` declarations committed as
`9c0ec57da37a301f26605b0423c413ed69b91735`. The first live reconcile exposed
a v1.0.7 panic when copying typed `int32` VM inputs into an unstructured
Workspace. That was fixed and regression-tested in `v1.0.8`, commit
`3253d7f4ce1a54fac3fd1407ab9c62033e7d4fb6`.

The v1.0.8 images were published successfully and the operator is running
with one ready replica. The root Argo Application is Synced/Healthy and no
longer tracks the legacy Workspace manifest. Datacenter adoption updated the
existing `infrastructure/homelab` Workspace from generation 5 to generation
6; it is Synced/Ready and reports VMID `201` at `10.10.10.21`. The Datacenter
and `web01` report Ready, and the first Datacenter-managed Ansible Job
(`web01-cc8ec54c-7c2169-1`) completed successfully. `Prune=false` remains on
the Workspace as an additional safeguard.

The live Workspace generation and outputs confirm that the complete `web01`
map reconciled and that the expected VM identity and address are present.
Provider logs did not expose an OpenTofu plan summary, so a zero-change/no-
replacement plan was not independently recorded.

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
- The live Workspace is `infrastructure/homelab`, generation 6, with
  `Synced=True` and `Ready=True`; it is no longer tracked by the root Argo
  Application. Its `web01` outputs report VMID `201` and address
  `10.10.10.21`.
- The live Datacenter and `web01` are Ready. The derived Configuration's
  applied revision matches its desired revision after the successful Job
  `web01-cc8ec54c-7c2169-1`.

Still to verify in the live POC:

- Capture an OpenTofu plan or equivalent provider evidence confirming the
  `module.vm` to `module.vms["web01"]` state move requires no VM replacement.
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
- The v1.0.7 GitHub Actions image-publish workflow succeeded, Argo CD rolled
  out the v1.0.7 controller before Datacenter adoption, and the new
  Datacenter and VirtualMachine CRDs were established.
- The initial live Datacenter reconcile on v1.0.7 panicked on `int32` VM
  values before changing the Workspace. v1.0.8 converts the VM core count and
  DNS list to JSON-compatible unstructured values; focused controller tests
  cover Workspace adoption, repeated no-op reconciliation, and these VM
  fields.
- The v1.0.8 GitHub Actions image-publish workflow succeeded, Argo CD rolled
  out the v1.0.8 controller with one ready replica, and the root Application
  is Synced/Healthy at commit `3253d7f4ce1a54fac3fd1407ab9c62033e7d4fb6`.
  The new Datacenter and VirtualMachine CRDs are established.
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
