---
id: REQ-RS-010
title: A crate's alternative registry travels with it
scope: rs
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Rust plugin **shall** report a crate from an alternative registry with
that registry: the name a `Cargo.toml` dependency gives (`registry =
"<name>"`, or its `registry-index` URL), else the index URL of the
`Cargo.lock` `source` it was locked from when that is not crates.io. The
transitive walk over `Cargo.lock` **shall** carry the same for each crate it
reports.

## Rationale

An alternative registry serves only the crates that declare it
([REQ-SUP-063](../sup/REQ-SUP-063-additive-sources-fall-back-to-the-public-index.md));
without this the index layer cannot tell them from crates.io's.

## Acceptance criteria

1. `billing = { version = "1", registry = "corp" }` resolves with registry
   `corp`; a crate locked from `sparse+https://crates.corp.example/index/`
   carries `https://crates.corp.example/index`; a crates.io crate carries
   none.
2. A crates.io dependency of a registry crate in `Cargo.lock` carries none.
