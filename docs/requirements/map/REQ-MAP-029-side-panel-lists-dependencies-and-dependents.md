---
id: REQ-MAP-029
uuid: 43778530-2a25-4523-bb80-8c8d2240e5b7
title: Side panel lists dependencies and dependents
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M1
verification:
  - manual
  - ui
---

## Statement

The side panel **shall** list what the selected node depends on and what depends
on it, one row per node, each row selecting and revealing that node when
activated.

## Rationale

The lists are the textual counterpart of the arcs and a way to walk the graph.

## Acceptance criteria

1. Selecting `internal/lang/golang` in this repository lists `go/parser` under
   Depends on and its importers under Used by.
2. Clicking a row selects that node and brings it into view.
