# Configuration resources

The `infra.example.io/v1alpha1` `Configuration` API describes an Ansible target
without tying the target to Proxmox-specific fields.

```yaml
apiVersion: infra.example.io/v1alpha1
kind: Configuration
metadata:
  name: web01
  namespace: infrastructure
spec:
  target:
    host: 10.10.10.21
    user: debian
    sshSecretRef: web01-ssh
    # Optional; use a separate Secret for the jump host's SSH key.
    proxyJump:
      host: 185.170.196.225
      user: root
      sshSecretRef: proxmox-ansible-jump
  infrastructureRef:
    workspaceName: homelab
  ansible:
    collections:
      - name: homelab.web
        version: 1.0.0
      - name: homelab.system
        version: 1.0.0
    roles:
      - name: homelab.web.nginx
        vars:
          nginx_page_content: Web is working!
      - name: homelab.system.textfile
    lifecycle:
      uninstallRemovedRoles: true
  reconciliation:
    onChange: true
    schedule:
      type: cron
      expression: "0 8 * * *"
      timezone: Europe/Vilnius
  execution:
    maxConcurrent: 10
    timeout: 30m
    maxRetries: 3
```

The example CR is stored at
[`operator/deploy/configuration-sample.yaml`](../operator/deploy/configuration-sample.yaml)
and is not synced automatically. `target.sshSecretRef` must refer to a Secret in
the Configuration namespace containing key `ssh-privatekey`. Avoid putting
secret values in role vars: Job playbooks are mounted from ConfigMaps.
When `target.proxyJump` is set, its separate Secret must also contain
`ssh-privatekey`; Jobs use that key only for the SSH tunnel to the jump host.
The Configuration includes jump host connection details in its desired
revision.

`infrastructureRef.workspaceName` is optional for physical hosts. When set, the
controller waits for that namespaced `opentofu.m.upbound.io/Workspace` to
report `Ready=True`. Target reachability is then checked by Ansible itself;
connection failures are bounded by the retry limit and Job deadline.

Status records `desiredRevision`, `appliedRevision`, attempted/successful/
completed/verification times, the active Job, result/error, applied roles and
collections, and Kubernetes conditions. A failed execution never updates
`appliedRevision`.

Useful commands:

```sh
kubectl -n infrastructure get configurations
kubectl -n infrastructure describe configuration web01
kubectl -n infrastructure get jobs -l infra.example.io/configuration=web01
kubectl -n infrastructure logs job/<execution-job-name>
```
