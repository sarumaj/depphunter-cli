---
id: REQ-MOD-007
uuid: 0d056947-85b1-4f64-b20d-00ed7277a62f
title: Reference edge kind
scope: mod
type: interface
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The graph model **shall** accept edges of kind `reference` (symbol or file to
symbol) without changes to the UI's logic that aggregates the edges of a
collapsed node.

## Rationale

Symbol references were planned from the first version as an optional layer;
declaring the kind in the model and keeping the aggregation independent of the
kind let them be added later without reworking the UI.

## Acceptance criteria

1. `reference` is a declared edge kind of the graph model.
2. The function that collects the edges crossing a collapsed node's boundary
   takes the edge kind as a parameter and applies the same logic to `import` and
   `reference` edges.

## Notes

How references are produced is scope `lsp`. References are served separately
from the graph document (`/api/references`).
