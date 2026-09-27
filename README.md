# kubectl-ns

A kubectl plugin to view or switch the namespace of the current KUBECONFIG context.

Built with Go 1.27, `k8s.io/cli-runtime` / `client-go` v0.37 and cobra v1.

## How it works

`kubectl ns <namespace>` takes the cluster and user of the current context and switches
`current-context` to a context named `<namespace>/<cluster>/<user>`, adding it if it
doesn't exist yet (`<user>` is cut at the first `/`; if that name is already taken by a
context with another cluster or user, the full user name is used). Other contexts are not touched.

Exception: with `--context NAME`, the namespace of context `NAME` is updated in place.

## Installing via Krew

This repository is its own [custom Krew index](https://krew.sigs.k8s.io/docs/user-guide/custom-indexes/):
every tagged release publishes `plugins/change-ns.yaml` to `master`.

```sh
kubectl krew index add macbet https://github.com/Macbet/kubectl-ns.git
kubectl krew install macbet/change-ns
kubectl krew upgrade   # later, to pick up new releases
```

Installed via Krew, the command is `kubectl change-ns` (`ns` in the default index is kubens).
Built from source, it is `kubectl ns`. Examples below use `ns`.

## Usage

```sh
# show the namespace that the current context points to
kubectl ns

# list all namespaces in use by contexts in your KUBECONFIG
kubectl ns --list

# switch to "new-namespace" (adds a context like new-namespace/<cluster>/<user>,
# existing contexts are left untouched)
kubectl ns new-namespace
```

`--kubeconfig`, `--context`, `--cluster` and `--user` are honored.

## Building from source

```sh
go install github.com/Macbet/kubectl-ns/cmd/kubectl-ns@latest
# or
go build -o kubectl-ns ./cmd/kubectl-ns && mv kubectl-ns ~/bin/  # anywhere in $PATH
```

## Releasing

```sh
git tag v1.0.0 && git push origin v1.0.0
```

The `release` workflow runs GoReleaser: builds linux/darwin/windows × amd64/arm64,
creates the GitHub release and commits the updated Krew manifest to `plugins/change-ns.yaml`.
If `master` is protected, add a `KREW_INDEX_TOKEN` secret (a PAT with `contents: write`
that may push to `master`); otherwise the built-in `GITHUB_TOKEN` is used.
In a fork, enable Actions in the repository's Actions tab first.
Local dry run: `goreleaser release --snapshot --clean`.

## Use Cases

This plugin can be used as a developer tool, in order to quickly view or change the current namespace
that kubectl points to.

It can also be used as a means of showcasing usage of the cli-runtime set of utilities to aid in
third-party plugin development.

This plugin's functionality is similar to that of the [oc project](https://github.com/openshift/origin/blob/master/docs/cli.md#oc-project) command.
This plugin has been tested against OpenShift and Kubernetes clusters using `kubectl`.

## Cleanup

```sh
kubectl krew uninstall change-ns
```
