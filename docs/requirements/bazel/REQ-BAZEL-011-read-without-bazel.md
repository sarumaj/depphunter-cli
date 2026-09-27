---
id: REQ-BAZEL-011
title: Bazel read without running Bazel
scope: bazel
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Bazel's files **shall** be read without running Bazel: macros are not
expanded (a target a macro declares is the macro call; label strings a
macro turns into labels, such as grpc's `external_deps = ["absl/base"]`, are
dropped), `select()` branches are all read, computed labels and URLs are
not evaluated, module extensions other than the listed hubs are not run
(their repositories go to the extension's module), and `.bazelrc` `import`
lines are not followed. A BUILD target's package is its directory from the
scanned root, not from a nested workspace. Only a lock file says which
version minimal version selection picked; without one a module is shown at
its declared minimum. Buck2 and other Starlark dialects are not read. OSV
has no Bazel ecosystem, so modules and WORKSPACE downloads are not asked
about; `--online` follows modules through registries (REQ-SUP-057), not
WORKSPACE repositories.

## Rationale

The map is built from files alone.

## Acceptance criteria

1. A label that names a repository no file of the workspace declares is an
   unresolved `bazel` package, not a guess.
