---
id: REQ-MAP-041
uuid: eb02873d-b7e0-4ca2-8d1f-57957db51002
title: Labels at both ends of selection arcs
scope: map
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

For a selection, the system **shall** label the selected node and the buildings,
symbols and packages at both ends of its arcs, before any region label.

## Rationale

The names at the ends of the arcs answer "what does this depend on" without
hovering each end.

## Acceptance criteria

1. Selecting a file labels it and the packages and files its arcs reach.
