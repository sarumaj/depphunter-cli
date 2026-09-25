---
id: REQ-LSP-004
uuid: dade0ca6-4aff-419f-91c0-ca5b7bef89b2
title: Reference credited to the enclosing definition
scope: lsp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M6
verification:
  - integration
---

## Statement

The system **shall** credit each reference location to the innermost definition
whose `textDocument/documentSymbol` extent contains it, or to the file when it
is in top-level code, falling back to the closest preceding definition when the
server gives no extents, and **shall** record it once as an edge of kind
`reference` from that definition (or file) to the referenced definition,
omitting self references.

## Rationale

A reference edge is only meaningful between the using and the used symbol.

## Acceptance criteria

1. A call inside a method body yields an edge from that method to the callee.
2. A reference in a variable declaration after a function body is credited to
   the variable, not the preceding function.
3. No edge leads from a symbol to itself.
