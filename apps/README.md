# Workloads

Place deployable Kubernetes YAML manifests under this directory. Manifests may
be organized into subdirectories; the root Argo CD Application recursively
discovers them.

Argo CD automatically syncs changes from `main`, corrects live drift, and
deletes managed resources when their manifests are removed from Git. Use
explicit namespaces in workload manifests where appropriate.
