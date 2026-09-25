---
id: REQ-RS-004
uuid: a8594d7f-bde0-45eb-a5df-c0a66e429109
title: Workspace and path dependencies resolve locally
scope: rs
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Rust plugin **shall** resolve a crate named by a `path` dependency, or by a
dependency whose package is a crate of the same repository (a workspace member
or any other `Cargo.toml` with that `[package] name`), to the file in that
crate's `src` directory the remaining path names, or to the crate's directory
when none does, rather than to crates.io.

## Rationale

A workspace's crates depend on each other through the repository, not through a
registry; drawing them as external packages would hide the internal structure.

## Acceptance criteria

1. `use core_lib::util` resolves to `core-lib/src/util.rs` in the test
   workspace.
2. Dashes and underscores in crate names are treated as equal (`core-lib` is
   `core_lib`).
3. Dependencies from `[dependencies]`, `[dev-dependencies]`,
   `[build-dependencies]` and `[target.*.…]` tables are all read.
