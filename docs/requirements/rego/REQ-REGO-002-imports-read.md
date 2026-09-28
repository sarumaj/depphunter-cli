---
id: REQ-REGO-002
title: Imports and references read
scope: rego
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every `import data.a.b` (with `as`, and bracketed segments
`data.a["b-c"]`) and every reference to `data.a.b...` in a rule, directly
or through an imported name, **shall** be read. `import input`, `import
rego.v1` and `import future.keywords...` **shall** not be imports, and
nothing in comments or strings **shall** be read.

## Rationale

Rego imports documents of `data`; a rule may also refer to a document
without importing it.

## Acceptance criteria

1. The fixture's `deny.rego` imports four data paths and none of the
   built-ins; its references resolve or vanish as REQ-REGO-004 states.
