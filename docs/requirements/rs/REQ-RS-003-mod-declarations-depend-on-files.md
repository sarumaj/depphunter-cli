---
id: REQ-RS-003
uuid: e2de9b31-df92-4b7b-acfb-f9674dc0db0a
title: Mod declarations depend on their files
scope: rs
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Rust plugin **shall** record every body-less `mod x;` declaration as a
dependency of the declaring file on the module file `x.rs` or `x/mod.rs` below
the declaring module's directory.

## Rationale

A `mod x;` declaration is what compiles another file into the crate, so the
declaring file depends on it even without any `use`.

## Acceptance criteria

1. `mod net` in `app/src/main.rs` resolves to `app/src/net/mod.rs`.
2. `mod config` in `app/src/main.rs` resolves to `app/src/config.rs`.
3. `mod server` in `app/src/net/mod.rs` resolves to `app/src/net/server.rs`.
4. A `mod x { … }` with an inline body is a symbol of kind `mod`, not a
   dependency.
