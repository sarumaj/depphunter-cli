---
id: REQ-JSONNET-004
title: Import resolution
scope: jsonnet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An import **shall** resolve as jsonnet finds the file: relative to the
importing file; then in the `vendor` and `lib` directories of each
jsonnet-bundler project (a directory with a `jsonnetfile.json`) above it,
nearest first; then in the `JSONNET_PATH` directories inside the
repository; then under an ancestor directory of the importing file. A file
found in a project's `vendor` directory **shall** be the package of the
dependency that installed it, named by its full path
(`github.com/org/repo/subdir/...`) or its legacy link (`vendor/<name>`).
When nothing is installed, the manifests **shall** name the package the
same way (the governing projects', then any project's of the repository);
a local source **shall** resolve to its files. A full path no manifest
names **shall** be an unresolved package named by its repository
(host/owner/repo on GitHub, GitLab, Bitbucket, Codeberg and sourcehut,
else host/first element), another path by its first element; a missing
bare or relative file **shall** be dropped.

## Rationale

jb writes dependencies to `vendor/` under their full path and links
legacy names beside them; tools (jsonnet `-J`, Tanka) put `vendor/` and
`lib/` on the library path.

## Acceptance criteria

1. The fixture's `ksonnet-util/kausal.libsonnet` (legacy link) and
   `github.com/grafana/jsonnet-libs/ksonnet-util/util.libsonnet` (full path)
   are ksonnet-util, doc-util is found in `vendor/` though only the lock
   names it, `k8s/main.libsonnet` and the full path of k8s-libsonnet are
   named by the manifest, `utils.libsonnet` is `lib/utils.libsonnet`,
   `extra.libsonnet` is found through `JSONNET_PATH`,
   `shared/lib.libsonnet` is the local source's file, and an unknown full
   path and an unknown name are unresolved packages.
