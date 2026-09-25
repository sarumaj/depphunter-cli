---
id: REQ-LSP-007
uuid: eabd76a4-e12b-4117-9d28-69cd9dba7fa8
title: Imports and References switch
scope: lsp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M6
verification:
  - manual
---

## Statement

Once references are available the UI **shall** offer an Imports / References
switch that chooses which kind of edge is drawn and listed for the selection.

## Rationale

Imports and symbol references answer different questions and would clutter each
other.

## Acceptance criteria

1. With references loaded the legend shows the switch; choosing References draws
   reference arcs for the selection.
2. Without references the switch is absent and imports are shown.
