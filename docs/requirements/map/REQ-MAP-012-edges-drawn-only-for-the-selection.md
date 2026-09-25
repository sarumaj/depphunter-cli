---
id: REQ-MAP-012
uuid: fb2f1fad-508c-4af2-a646-7866da470794
title: Edges drawn only for the selection
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §2
verification:
  - ui
  - manual
---

## Statement

The system **shall not** draw any dependency edge while nothing is selected;
when a node is selected it **shall** draw the edges that cross the boundary of
the selected node's subtree, and no others, up to 400 aggregated arcs, largest
counts first.

## Rationale

Drawing every edge of a repository at once produces an unreadable hairball;
edges are an answer to a question about one node.

## Acceptance criteria

1. With no selection, the map shows no arcs.
2. Selecting a node shows only arcs with that node's representative at one end.
3. Clearing the selection removes all arcs.
