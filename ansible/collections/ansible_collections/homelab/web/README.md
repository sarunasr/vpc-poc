# homelab.web

Versioned Ansible collection used by the Infrastructure GitOps POC.

## Roles

- `homelab.web.nginx`: manage Nginx and a POC landing page on Debian 13.

Build the collection reproducibly with `make collections-build`. The version in
`galaxy.yml` is the immutable declared collection version; bump it when role
content changes that should be deployed.
