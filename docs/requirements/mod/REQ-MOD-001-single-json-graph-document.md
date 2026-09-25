---
id: REQ-MOD-001
uuid: 94d60dbd-5586-4e08-8d6a-a91844e8e166
title: Single JSON graph document
scope: mod
type: interface
priority: must
status: implemented
verification:
  - unit
  - inspection
---

## Statement

The system **shall** represent the result of an analysis as one JSON document
with the members `root` (the name of the analyzed directory), `generatedAt` (an
RFC 3339 UTC timestamp), `nodes` (an array of nodes) and `edges` (an array of
edges), and **shall** use that same document as the input of the UI, the exports
and the VS Code extension.

## Rationale

One document exchanged between analysis and every consumer keeps the consumers
independent of how the analysis was carried out and lets each consumer be tested
against a fixed shape.

## Acceptance criteria

1. The graph served to the UI parses as JSON and has exactly the top-level
   members `root`, `generatedAt`, `nodes` and `edges`.
2. `generatedAt` is a UTC timestamp in RFC 3339 format.
3. `nodes` and `edges` are arrays, empty rather than absent when there is
   nothing to list.

## Notes

The JSON export writes the same document (scope `exp`). The member names are
declared once, in `internal/graph/graph.go`.
