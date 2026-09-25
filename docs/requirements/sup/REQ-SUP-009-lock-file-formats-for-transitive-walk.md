---
id: REQ-SUP-009
uuid: a17a7ea2-4bd4-4cb3-b486-eff9535d32f7
title: Lock files read for the transitive walk
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The walker **shall** read package-to-package dependencies from
`package-lock.json` versions 1 to 3, `pnpm-lock.yaml` versions 5 to 9, classic
`yarn.lock`, `Cargo.lock`, `uv.lock`, `poetry.lock` and `pdm.lock`.

## Rationale

Each of these files already records the whole resolved graph, each in its own
shape, so the walk needs nothing but the repository.

## Acceptance criteria

1. For each listed format, a fixture yields the dependencies the file records
   for a package, with the locked version and pinned.
2. A crate present in two versions in `Cargo.lock` is resolved to the version
   the lock names.
