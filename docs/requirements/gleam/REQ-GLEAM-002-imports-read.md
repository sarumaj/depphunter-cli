---
id: REQ-GLEAM-002
title: Imports read
scope: gleam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Gleam plugin **shall** read every `import a/b/c` of a module as an import
of the module path `a/b/c`, whether it is followed by an unqualified list
(`.{type T, f as g, C}`), an alias (`as c`) or both, and **shall** not read
imports inside strings or comments.

## Rationale

Imports are the only way one Gleam module uses another.

## Acceptance criteria

1. `import gleam/dict.{type Dict} as d` in the fixture's `src/shop.gleam` is
   the import `gleam/dict`.
2. `import not/this` inside a string constant and `// import commented/out`
   yield nothing.
