---
id: REQ-BAZEL-008
title: MODULE.bazel.lock read
scope: bazel
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The `MODULE.bazel.lock` beside a MODULE.bazel **shall** give the version
each module was selected at: the highest version among the registry files
the resolution fetched (`registryFileHashes`, Bazel 7.2 and later), or the
resolved module graph of older lock files (`moduleDepGraph`), which **shall**
also answer what a module depends on (`lang.Transitive`). Versions compare
as Bazel's: numeric segments as numbers, a pre-release before its release.

## Rationale

The declared version is a minimum; the lock says what was built.

## Acceptance criteria

1. rules_cc declared 0.0.9 with 0.0.9, 0.0.10-rc1 and 0.0.10 in
   registryFileHashes is 0.0.10 requested 0.0.9.
2. An older lock's graph gives rules_go 0.42.0 (declared 0.41.0) and its
   dependency bazel_skylib 1.4.1, and not bazel_tools.
