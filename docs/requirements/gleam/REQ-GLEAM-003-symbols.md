---
id: REQ-GLEAM-003
title: Definitions extracted
scope: gleam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Gleam plugin **shall** extract a module's top-level functions (`fn`,
`pub fn`, bodiless `@external` functions and pre-0.30 `external fn`) as
`func`, custom types, opaque types and type aliases as `type`, their
constructors as `Type.Constructor` of kind `constructor`, and constants as
`const`; and a `gleam.toml`'s package name as `package`.

## Rationale

These are what other modules import and call.

## Acceptance criteria

1. `src/shop.gleam` yields `Order`, `Order.Order`, `Order.Cancelled`,
   `Token`, `Token.Token`, `Stock`, `version`, `greeting`, `main` and
   `helper` with their kinds and lines.
2. Anonymous functions (`fn(x) { .. }`) inside bodies are not symbols.
