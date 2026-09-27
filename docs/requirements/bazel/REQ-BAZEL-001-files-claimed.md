---
id: REQ-BAZEL-001
title: Bazel files claimed
scope: bazel
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The bazel plugin **shall** claim Bazel's Starlark files: `BUILD` and
`BUILD.bazel`, `.bzl` files, `MODULE.bazel` and the segments it
`include()`s (`*.MODULE.bazel`), and `WORKSPACE`, `WORKSPACE.bazel` and
`WORKSPACE.bzlmod`, outside Bazel's output trees (`bazel-bin`, `bazel-out`,
`bazel-testlogs`, `bazel-genfiles`). The file kind, and for a BUILD file its
directory, **shall** be part of the cache key. `MODULE.bazel.lock`,
`.bazelrc` and generic Starlark (`.star`, Buck2's `BUCK`, Tilt's
`Tiltfile`) are not claimed.

## Rationale

`MODULE.bazel` and `BUILD.bazel` share an extension, and a BUILD file's
targets are named by its package, so the same content in another directory
declares other targets.

## Acceptance criteria

1. `pkg/BUILD.bazel`, `defs.bzl`, `deps.MODULE.bazel` and
   `WORKSPACE.bzlmod` are claimed; `MODULE.bazel.lock`, `.bazelrc`,
   `x.star` and `bazel-out/k8/bin/x/BUILD` are not.
2. The same BUILD content in two directories, or in a BUILD and a WORKSPACE
   file, has different cache keys.
