---
id: REQ-GLEAM-001
uuid: a42946bb-0139-4690-acbc-797e740bbac4
title: Files claimed
scope: gleam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Gleam plugin **shall** claim Gleam modules (`.gleam`), `gleam.toml` and
`manifest.toml` (in lower case, so Julia's `Manifest.toml` is not taken),
telling the two TOML files apart from other TOML files by name. What gleam
downloads and compiles into `build/packages/`, `build/dev/`, `build/prod/`
and `build/lsp/` **shall** not be analyzed; a `build/` directory of any other
layout is left alone. A `manifest.toml` Gleam did not write (no package lists
build tools and there is no `[requirements]` table) yields nothing.

## Rationale

A Gleam package is described by its modules and these two files; the build
directory holds copies of dependencies and compiled output, not the project.

## Acceptance criteria

1. `src/app.gleam`, `gleam.toml` and `x/manifest.toml` are claimed;
   `Manifest.toml` and `Cargo.toml` are not.
2. `build/packages/gleam_stdlib/src/gleam/list.gleam` and
   `build/dev/erlang/app/_gleam_artefacts/app.gleam` are not claimed;
   `src/build/tool.gleam` is.
3. A `manifest.toml` with only a `[package]` table extracts no imports.
