# Troubleshooting

## Argo CD

```sh
kubectl -n argocd get applications
kubectl -n argocd describe application root
```

Provider custom resources can show a temporary missing-CRD error until the
Crossplane Helm Application becomes healthy.

## Crossplane / OpenTofu

```sh
kubectl get providers.pkg.crossplane.io
kubectl -n crossplane-system get pods
kubectl -n infrastructure get workspaces.opentofu.m.upbound.io
kubectl -n infrastructure describe workspace homelab
kubectl -n crossplane-system logs deploy/provider-opentofu-<provider-id>
```

Inspect the actual provider Deployment name and ServiceAccount after package
installation. `Pending` credentials usually mean the Proxmox API token Secret,
ProviderConfig, or BPG permissions are missing. A stuck Workspace can be an
OpenTofu timeout, a stale backend Lease, or Proxmox API/TLS error. Inspect
Workspace conditions/logs, the state Secret and Lease before retrying. Never
delete the state Secret to clear an error.

## Configuration / Ansible

```sh
kubectl -n infrastructure describe configuration web01
kubectl -n infrastructure get jobs,pods
kubectl -n infrastructure logs job/<job-name>
kubectl -n infrastructure describe job <job-name>
kubectl -n infrastructure logs deploy/configuration-controller
```

- `CollectionVersionUnavailable`: the image and pinned CR versions differ;
  build/publish the intended immutable Runner image and update the controller's
  version declaration.
- `InfrastructureNotReady`: inspect the referenced Workspace `Ready` condition.
- SSH errors: verify the Secret contains `ssh-privatekey`, target host/user,
  target reachability from the Job Pod, and target `debian` sudo access.
- `AnsibleExecutionFailed`: inspect Job logs, then correct the desired input or
  create a new manual request after bounded retries are exhausted.
- `CleanupUnsupported`: expected for removed roles with unsupported metadata;
  no cleanup is attempted.
- A TLS error to Proxmox should be fixed by using a trusted certificate. The
  lab's self-signed certificate may use the documented insecure switch only
  for development.

The initial VM template installs `qemu-guest-agent`; the Terraform output
address requires that agent to be active. Verify Proxmox task completion and
guest initialization before diagnosing target SSH.
