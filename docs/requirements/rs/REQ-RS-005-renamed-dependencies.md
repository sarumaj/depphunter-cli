---
id: REQ-RS-005
uuid: 44deb02e-5946-4fd1-838e-9bc84063a3b8
title: Renamed Cargo dependencies
scope: rs
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Rust plugin **shall** resolve a dependency declared under a local name with
`package = "<name>"` to the crates.io package `<name>`, while matching imports
by the local name.

## Rationale

Cargo lets a manifest rename a dependency; the code uses the local name, but the
package, its version and its advisories belong to the real name.

## Acceptance criteria

1. With `json = { package = "serde_json", version = "1" }`, `use json::Value`
   resolves to package `serde_json` in the `crates` ecosystem.
