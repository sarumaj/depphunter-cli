---
id: REQ-RS-002
uuid: 242f923e-fe55-4aae-bdad-1a3f9d1ae837
title: Module paths resolve to module files
scope: rs
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

The Rust plugin **shall** resolve a path beginning with `crate::` from the `src`
directory of the file's crate, a path beginning with `self::` from the directory
of the current module, and a path beginning with one or more `super::` from the
corresponding ancestor module directory, to the file of the longest prefix of
the path that names a module (`<name>.rs` or `<name>/mod.rs`), or to the module
root file (`lib.rs`, `main.rs`, `mod.rs`, or the 2018-layout `<dir>.rs`) when no
segment remains.

## Rationale

Module files are where a Rust crate's internal dependencies lie; the module
tree, not the directory tree, decides which file a path names.

## Acceptance criteria

1. `use crate::net` in `app/src/main.rs` resolves to `app/src/net/mod.rs`.
2. `use crate::net::server::Server` resolves to `app/src/net/server.rs`.
3. `use super::config::Settings` in `app/src/net/mod.rs` resolves to
   `app/src/config.rs`.
4. `use super` in `app/src/net/server.rs` resolves to `app/src/net/mod.rs`.
5. A path whose first segment names a module of the current file (edition 2018)
   resolves to that module file.

## Notes

`lib.rs`, `main.rs` and `mod.rs` own their directory; any other `a/b.rs` owns
`a/b/`. A path starting with an upper-case segment (`use Enum::*`) names a local
item and resolves to nothing.
