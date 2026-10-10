# Ansible collections and role lifecycle

Both collections follow the normal Galaxy collection structure:

- `ansible/collections/ansible_collections/homelab/web`
- `ansible/collections/ansible_collections/homelab/system`

Each has `galaxy.yml`, README, role defaults/tasks/meta/tests. Build immutable
versioned archives and the Runner image:

```sh
make collections-build
make collections
make ansible-image
```

Role changes intended for deployment require a collection version bump in
`galaxy.yml`, the Runner image's `requirements.yml`/build, and the controller's
`--collection-versions` manifest. Rebuild, publish a new immutable image tag,
and update the operator deployment. The operator rejects Configuration
collection versions that are not baked into its declared execution image.
Updating Git role source without a collection/image version change does not
silently change deployed code.

`homelab.web.nginx` installs Nginx on Debian, writes only its managed
`index.html`, and supports `nginx_state: absent`. Its custom metadata at
`roles/nginx/meta/gitops.yml` declares the uninstall variable. When a previously
applied role is removed and `uninstallRemovedRoles` is true, the controller
reads that metadata from the execution image and adds an uninstall invocation
to the next Job. Cleanup is skipped if the previous collection version is no
longer the pinned image version.

`homelab.system.textfile` writes `/opt/gitops-demo/hello.txt` and declares
uninstall unsupported. Removing it never deletes the file; a
`RoleCleanup=False, reason=CleanupUnsupported` condition explains the skip.
When `uninstallRemovedRoles` is false, removed roles are simply no longer run.

Only cleanup described by a role's project metadata is invoked. The operator
does not infer destructive behavior from arbitrary Ansible tasks, and cleanup
failure does not delete a VM. Collection artifact retention is necessary if
older-version uninstall must be supported after upgrades; the initial image
build contains the two declared collection versions.
