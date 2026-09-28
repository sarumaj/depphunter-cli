---
id: REQ-SUP-057
title: Bazel registries for module dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a module of the
`bazel` island depends on from its registry's files:
`<registry>/modules/<name>/<version>/MODULE.bazel` at the version the
target names, else the newest version `metadata.json` lists that is not
yanked; its `bazel_dep` calls without `dev_dependency = True` are the
dependencies, each at the version it names, which pins. The Bazel Central
Registry (`https://bcr.bazel.build`) is the public index; registries a
`.bazelrc` names with `--registry` **shall** be recorded as the
repository's (the project's `.bazelrc` files) or this machine's
(`~/.bazelrc`), except the Bazel Central Registry itself and `file://`
registries. The files a `.bazelrc`'s `import`, `try-import` and
`try-import-if-bazel-version` lines name **shall** be read where the line
stands: `%workspace%` is the workspace's directory (the nearest directory
with `MODULE.bazel`, `REPO.bazel`, `WORKSPACE` or `WORKSPACE.bazel`, else the
repository's root; none for `~/.bazelrc`, whose `%workspace%` lines are
skipped); another relative path is taken from the workspace, as Bazel run
there does, else from the importing file's directory. A repository's
`.bazelrc` imports no file outside the repository; a missing file and an
import loop are passed over.

## Rationale

A registry is a file tree; a module's MODULE.bazel is its manifest.

## Acceptance criteria

1. rules_go 0.50.1 yields bazel_features, platforms and protobuf at their
   versions, not the dev dependency gazelle; without a version, 0.51.0 is
   asked (0.52.0 is yanked).
2. A project `.bazelrc` naming BCR, an internal registry and a `file://`
   one records only the internal registry, untrusted; `~/.bazelrc` is
   trusted.
3. Registries of imported files (`%workspace%`, workspace- and
   file-relative, a file outside the scan, a versioned import) come in the
   order of the import lines; a missing file, a loop and a file outside the
   repository add nothing; `~/.bazelrc`'s import is this machine's.

## Notes

The sandbox proxy blocks bcr.bazel.build, so the Bazel Central Registry was
verified with a stub server only; envoy's own registry, which its `.bazelrc`
names on raw.githubusercontent.com, is reachable.
