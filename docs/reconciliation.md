# Reconciliation and execution

The controller hashes the declarative Configuration spec to derive
`desiredRevision`. Collection versions, role order/vars, target details,
schedule, and execution limits participate in the hash; unrelated repository
commits do not.

## Triggers

- **On change:** `reconciliation.onChange: true` starts a Job when the desired
  revision differs from `status.appliedRevision`.
- **Interval:** `schedule.type: interval` with a Go duration such as `24h`.
- **Cron:** `schedule.type: cron`, a five-field cron expression, and an IANA
  timezone such as `Europe/Vilnius`. DST uses the named timezone.
- **Manual:** change the request annotation value; a request is recorded and
  runs only once.

```sh
kubectl -n infrastructure annotate configuration web01 \
  infra.example.io/reconcile-request="$(date +%s)" --overwrite
```

Only interval or cron is configured at a time. `onChange` may accompany a
schedule. At most one missed schedule occurrence is run after downtime; future
intervals are measured from the last successful run. A periodic failure is
bounded by `maxRetries`; the next schedule occurrence is still eligible.

## Jobs and recovery

Every execution uses one deterministic Job per Configuration/revision/trigger
attempt. A restart discovers the Job from status and does not create another
while it is running. Jobs have no in-Pod retries; the operator applies bounded
retries with capped backoff. Finished Jobs are kept for one day for logs.
Duplicate active work for a target is suppressed, and
`execution.maxConcurrent` caps active Jobs in the namespace.

The Runner receives a generated inventory and playbook through a ConfigMap and
the SSH private key through a read-only Secret volume. Target facts/variables
are not printed by the controller. Job deadline defaults to 30 minutes.

Configuration drift is not continuously observed by Ansible. Periodic runs
reapply the declared roles; this is eventual configuration correction, not
real-time drift detection.

## Infrastructure

Crossplane/provider-opentofu owns infrastructure `tofu plan`/`apply`, drift
reconciliation, and remote state. The operator only waits on Workspace
readiness; it does not run OpenTofu or share state.
