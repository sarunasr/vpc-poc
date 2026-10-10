# vpc-poc GitOps

This repository is the source of truth for workloads deployed to the MicroK8s
cluster. Argo CD runs in the `argocd` namespace and the root Application
watches `apps/` for Kubernetes manifests.

## Bootstrap

Argo CD is installed separately from the workload repository. To install it on
the configured cluster:

```sh
KUBECONFIG="$HOME/.kube/config-vpc-poc"
kubectl --kubeconfig "$KUBECONFIG" create namespace argocd \
  --dry-run=client -o yaml | kubectl --kubeconfig "$KUBECONFIG" apply -f -
kubectl --kubeconfig "$KUBECONFIG" apply --server-side --force-conflicts -n argocd \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/v3.5.4/manifests/install.yaml
kubectl --kubeconfig "$KUBECONFIG" -n argocd rollout status deployment/argocd-server \
  --timeout=5m
```

The repository is private, so Argo CD uses a dedicated SSH deploy key with
read-only access. The `repo-vpc-poc` Secret has already been created in the
cluster. Add the public deploy key provided during setup in the repository's
GitHub settings, without enabling write access. Never commit the private key.

To create or rotate the key, generate it outside the repository, register its
public half as a read-only GitHub deploy key, then replace the cluster Secret:

```sh
KEY_DIR="$(mktemp -d)"
ssh-keygen -t ed25519 -N '' -C 'argocd-vpc-poc-readonly' -f "$KEY_DIR/repo-key"
cat "$KEY_DIR/repo-key.pub"
# Add the printed key in GitHub before continuing.
kubectl --kubeconfig "$HOME/.kube/config-vpc-poc" -n argocd \
  create secret generic repo-vpc-poc \
  --from-file=sshPrivateKey="$KEY_DIR/repo-key" \
  --from-literal=type=git \
  --from-literal=url=ssh://git@github.com/sarunasr/vpc-poc.git \
  --dry-run=client -o yaml | kubectl --kubeconfig "$HOME/.kube/config-vpc-poc" \
  apply --server-side -f -
kubectl --kubeconfig "$HOME/.kube/config-vpc-poc" -n argocd \
  label secret repo-vpc-poc argocd.argoproj.io/secret-type=repository
rm "$KEY_DIR/repo-key" "$KEY_DIR/repo-key.pub"
rmdir "$KEY_DIR"
```

After adding the deploy key and pushing this repository's initial commit to
`main`, apply the root Application:

```sh
kubectl --kubeconfig "$HOME/.kube/config-vpc-poc" apply \
  -f bootstrap/argocd/root-application.yaml
```

The root Application automatically syncs manifests, corrects drift, and prunes
resources whose manifests are removed from `apps/`. Keep only resources you
intend Argo CD to manage under that directory.

## Argo CD access

The Argo CD server remains a ClusterIP service. To access the UI locally:

```sh
kubectl --kubeconfig "$HOME/.kube/config-vpc-poc" -n argocd \
  port-forward svc/argocd-server 8080:443
```

Open <https://localhost:8080>. Retrieve the initial admin password with:

```sh
kubectl --kubeconfig "$HOME/.kube/config-vpc-poc" -n argocd \
  get secret argocd-initial-admin-secret \
  -o jsonpath='{.data.password}' | base64 --decode; echo
```

Change the initial password after signing in.

## Adding services

Add Kubernetes YAML manifests anywhere under `apps/` and push them to `main`.
Argo CD recursively discovers and applies those manifests. See
[`apps/README.md`](apps/README.md) for details.
