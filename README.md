# kubectl-ns

A kubectl plugin to view or switch the namespace of the current KUBECONFIG context.

Built with Go 1.27, `k8s.io/cli-runtime` / `client-go` v0.37 and cobra v1.

## How it works

`kubectl ns <namespace>` takes the cluster and user of the current context and switches
`current-context` to a context named `<namespace>/<cluster>/<user>`, adding it if it
doesn't exist yet. `<user>` is cut at the first `/` (OpenShift-style `user/cluster` names).
Other contexts are never overwritten: if that name belongs to a context with different
settings, the full user name is tried, then a `-2`, `-3`, … suffix.

`--cluster` / `--user` replace the cluster / user taken from the current context.

`--context NAME` is the exception that edits in place: context `NAME` gets the new
namespace (and `--cluster` / `--user`, if given) and becomes `current-context`.
If `NAME` doesn't exist, it is created from the current context's cluster and user.

## Installing via Krew

This repository is its own [custom Krew index](https://krew.sigs.k8s.io/docs/user-guide/custom-indexes/):
each release publishes `plugins/change-ns.yaml` to `master`.

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
kubectl ns --context other   # ...or that another context points to

# list all namespaces in use by contexts in your KUBECONFIG
kubectl ns --list

# switch to "new-namespace" (adds a context like new-namespace/<cluster>/<user>,
# existing contexts are left untouched)
kubectl ns new-namespace
```

`--kubeconfig` and `$KUBECONFIG` (including multiple files) are honored; changes are
written back to the file the current context came from.

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

- Pre-release tags (`v1.1.0-rc.1`) become GitHub pre-releases and are not published to the index.
- A tag older than the newest release (`v1.0.5` after `v1.1.0`) is released but doesn't
  roll the index back.
- Push one tag at a time: queued release runs of several tags pushed together may be cancelled.
- If `master` is protected, add a `KREW_INDEX_TOKEN` secret (a PAT with `contents: write`
  that may push to `master`); otherwise the built-in `GITHUB_TOKEN` is used.
- In a fork, enable Actions in the repository's Actions tab first.

Local dry run: `goreleaser release --snapshot --clean`.

## Cleanup

```sh
kubectl krew uninstall change-ns
```
