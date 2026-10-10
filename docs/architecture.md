# Architecture

```text
Git -> Argo CD -> Crossplane 2 -> provider-opentofu -> OpenTofu/BPG -> Proxmox
                         |
                         +-> Configuration CR -> Go operator -> Job -> Ansible Runner -> VM
```

Argo CD remains the only GitOps reconciler. Crossplane owns the OpenTofu
Workspace and Terraform state; the custom controller does not implement a
Terraform engine. The custom operator observes Workspace readiness and owns
Configuration resources, their execution ConfigMaps/Jobs, and status. Each
Configuration Job runs one immutable Ansible execution image against a target
using a Kubernetes SSH Secret.

Infrastructure reconciliation and configuration reconciliation are separate.
A `Configuration` may reference an OpenTofu Workspace; it will not start
Ansible until that Workspace reports `Ready=True`. Static targets can omit
that reference for future physical-host support.

Ansible runs for a changed effective Configuration hash, a new manual request,
or one periodic schedule. Pinned collection names/versions are part of the
hash. A periodic run uses the same declared image and does not upgrade a
collection. Cron expressions use the configured IANA timezone. After downtime,
the controller schedules at most one overdue occurrence, then schedules from
the most recent successful run.

The operator persists applied revisions, roles, collections and execution
timestamps in Kubernetes status. Deterministic Job names and `ActiveJob`
status let it rediscover running Jobs after restart. Failed jobs do not advance
the applied revision. Retries are bounded; set a new desired revision or issue
a new manual request after the limit is reached.

See [reconciliation](reconciliation.md), [role lifecycle](ansible-lifecycle.md)
and [installation](installation.md).
