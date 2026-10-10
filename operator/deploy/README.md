# Operator deployment resources

Argo CD's root Application syncs the CRD and controller resources in this
directory once the operator image has been published to GHCR. The sample
Configuration is intentionally not auto-synced: create it only after the
Proxmox Workspace, VM, SSH credential Secret, and execution image are ready.
