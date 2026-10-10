# Infrastructure GitOps POC

A Kubernetes-native GitOps proof of concept for infrastructure and operating
system configuration. Argo CD syncs this repository; Crossplane with
`provider-opentofu` owns Terraform/OpenTofu infrastructure reconciliation; a
small Go operator coordinates Ansible Runner Jobs for target configuration.

This repository does **not** install or replace the existing MicroK8s, Argo CD,
Proxmox, Debian VM template, or network. For deployment assumptions and the
current validation status, see [`docs/installation.md`](docs/installation.md)
and [`docs/poc-tests.md`](docs/poc-tests.md).

## Current POC status

Implemented in Git:

- Terraform VM module for the existing Debian 13 Cloud-Init template,
  `local` storage, and `vmbr0`, with KVM acceleration not requested and
  `prevent_destroy` protection.
- Independently versioned `homelab.web` and `homelab.system` Ansible
  collections and an Ansible Runner execution image build.
- Argo CD declarations for Crossplane 2.4.2 and provider-opentofu 1.1.9.
- A Go Configuration controller, CRD, scoped Kubernetes permissions, and
  Ansible Job creation with deterministic revisions, interval/cron/manual
  triggers, retry limits, and per-host/overall concurrency controls.

Not yet end-to-end verified:

- Proxmox provisioning and state recovery: the Proxmox API token authenticates,
  but its ACL currently grants VM administration on protected VMIDs `101` and
  `102` and template `9000`. No Workspace or VM has been applied; narrow and
  verify the Proxmox ACL before applying one.
- The homelab POC accepts provider-opentofu's existing cluster-wide wildcard
  access to Kubernetes Secrets, ConfigMaps and Leases. This is a known security
  limitation and must be addressed before production.
- Private module access is prepared using Argo CD's read-only SSH deploy key,
  staged with SSH-safe permissions and mounted read-only into
  provider-opentofu. The running provider still needs to be rolled out with
  this runtime configuration and tested against the private repository before
  a Workspace can be created.
- The operator and Crossplane/provider are installed and running. The images
  are public; anonymous registry access uses GHCR's bearer-token challenge.
  The `1.0.2` controller rollout is complete. The Git repository remains
  private, but published image contents are pullable by anyone.
- Nginx/textfile execution, role-removal cleanup, version upgrade, and full
  infrastructure lifecycle tests require a provisioned test VM and its SSH
  Secret. No VM is created or removed by this initial setup.

The local verification status and blockers are documented in
[`docs/poc-tests.md`](docs/poc-tests.md). Do not treat YAML presence as proof
that a feature is operational.

## Repository map

- [`apps/`](apps): Argo CD-managed Kubernetes declarations.
- [`terraform/`](terraform): reusable Proxmox VM module and homelab root module.
- [`crossplane/`](crossplane): Crossplane/provider setup and infrastructure
  Workspace configuration guidance.
- [`operator/`](operator): Go Configuration API/controller and deploy manifests.
- [`ansible/`](ansible): versioned collections, tests, and Runner image.
- [`docs/`](docs): architecture, safety, installation and testing.

## Build and validate

Required local tools: Go 1.25+, Terraform/OpenTofu, Ansible Core/Galaxy, Docker.
The following commands do not access Proxmox or change the Kubernetes cluster:

```sh
make test
make terraform-validate
make ansible-image
make operator-image
```

To install the POC after publishing the two images and supplying a dedicated
Proxmox API token, see [`docs/installation.md`](docs/installation.md).

## Safety

- Use only the dedicated Proxmox API token; do not use root credentials.
- No passwords, API tokens, or SSH private keys belong in Git.
- Workspace deletion policy is retain/orphan by default; Terraform VM resources
  also have `prevent_destroy`.
- VMs are not provisioned until credentials and a reviewed Workspace manifest
  are present.
- Keep the existing template VMID `9000`, networking and VMIDs `101`/`102`
  untouched.
- The OpenTofu Kubernetes backend uses Secrets for state and Leases for locks;
  back up the `infrastructure` namespace accordingly.
- provider-opentofu's cluster-wide Kubernetes Secret, ConfigMap and Lease
  access is accepted for this personal homelab POC only; do not carry this
  permission model into production without a security review and remediation.

## Provider choice

OpenTofu is the main POC path because a Terraform root can manage multiple
provider types in one persistent state. A native Proxmox Crossplane provider
is evaluated separately in [`docs/provider-comparison.md`](docs/provider-comparison.md);
it is not installed and must not own a VM also managed by OpenTofu.
