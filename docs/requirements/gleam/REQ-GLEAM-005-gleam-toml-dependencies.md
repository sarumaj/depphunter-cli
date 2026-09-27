---
id: REQ-GLEAM-005
uuid: 48e575ee-bb46-4274-9876-ed67d9e0efab
title: gleam.toml dependencies as imports
scope: gleam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Gleam plugin **shall** read `gleam.toml`'s `[dependencies]` and
`[dev-dependencies]` (also `dev_dependencies` and inline tables), each entry
a Hex requirement, `{ version = .. }`, `{ path = .. }` or
`{ git = .., ref = .. }`, as an import of the package it names on its line; a
path dependency is an edge to that package's `gleam.toml`.

## Rationale

What a package declares is on the map even when no module imports it yet.

## Acceptance criteria

1. The fixture's `gleam.toml` imports `inventory` as
   `libs/inventory/gleam.toml` and `gleeunit` on line 20.
2. `tools/cli/gleam.toml` reads `gleeunit` from an inline
   `dev-dependencies = { .. }` table.
