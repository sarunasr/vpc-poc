# Optional Configuration samples

The controller's CRDs, RBAC, and Deployment are managed from
[`apps/operator`](../../apps/operator). VirtualMachine resources now generate
their own Configuration resources; do not apply a standalone Configuration
for a VM already managed by a VirtualMachine.

These files remain as examples for independent/static Ansible targets and are
not synchronized by Argo CD. Configure their SSH Secrets and infrastructure
readiness before applying one manually.
