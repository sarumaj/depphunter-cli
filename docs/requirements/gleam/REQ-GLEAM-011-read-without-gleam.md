---
id: REQ-GLEAM-011
uuid: aaf9bd50-5f34-43a1-bc64-67188ce40489
title: Read without running gleam
scope: gleam
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Gleam **shall** be read without running gleam or downloading packages: a
module of a package that no `gleam.toml` or `manifest.toml` declares, and
that is not in `build/packages/` on disk, is named by its leading segment
(the naming convention, not a registry lookup); pre-1.0 conditional blocks
(`if erlang { .. }`) and `@target(..)` are not read, so both targets'
definitions and imports count; an Elixir module named in `@external` is
dropped; `node:` specifiers in JavaScript externals are dropped.

## Rationale

Asking hex.pm which package provides a module needs the network and a
package-interface lookup per module; the files alone name nearly all of them.

## Acceptance criteria

1. `nothere/thing` is the unresolved package `nothere` and
   `@external(erlang, "Elixir.Jason", ..)` is dropped.
