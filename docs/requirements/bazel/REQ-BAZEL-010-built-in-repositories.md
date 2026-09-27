---
id: REQ-BAZEL-010
uuid: 3a9586b3-deb4-425f-ad1f-b277d9b8c521
title: Bazel's own repositories
scope: bazel
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Repositories Bazel provides itself (`bazel_tools`, `local_config_*`,
`local_jdk`, `remotejdk*`, `remote_java_tools*`, `remote_coverage_tools`,
`_builtins`) **shall** resolve to the hidden `bazel-std` island, one node per
repository.

## Rationale

Every WORKSPACE loads `@bazel_tools//tools/build_defs/repo:http.bzl`; it is
Bazel, not a dependency.

## Acceptance criteria

1. `@bazel_tools//tools/cpp:toolchain` is `bazel_tools` of `bazel-std`.
