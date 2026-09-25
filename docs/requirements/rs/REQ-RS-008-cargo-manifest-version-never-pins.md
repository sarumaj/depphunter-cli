---
id: REQ-RS-008
uuid: a9d79da2-cf2b-49dd-a057-66dba1a86170
title: A Cargo.toml version alone never pins
scope: rs
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The Rust plugin **shall** treat a version in `Cargo.toml` as a caret range, and
**shall** mark a crates.io dependency as pinned only when `Cargo.lock` fixes its
version.

## Rationale

Cargo reads a bare `1.2.3` as `^1.2.3`; the same string that pins in `go.mod`
floats in `Cargo.toml`. What counts as a pin is the ecosystem's own rule, and
for Cargo only the lock file decides.

## Acceptance criteria

1. `anyhow = "1.0.86"` with no `Cargo.lock` entry resolves to version `1.0.86`,
   not pinned (floating).
2. Removing `Cargo.lock` makes every crates.io dependency of the repository
   floating.

## Notes

The general floating concept and its presentation belong to scope `sup`.
