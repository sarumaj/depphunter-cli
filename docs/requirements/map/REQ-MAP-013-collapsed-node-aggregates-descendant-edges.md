---
id: REQ-MAP-013
uuid: e2410747-79c9-448c-9d06-512474957059
title: Collapsed node aggregates descendant edges
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

The system **shall** show, for a selected directory, the edges of all its
descendants that leave or enter its subtree, aggregated into one arc per pair of
representatives and direction, each carrying the number of edges it stands for.

## Rationale

A collapsed node stands for everything beneath it, so its dependencies are the
union of its descendants', counted.

## Acceptance criteria

1. Selecting a directory whose three files import the same package draws one
   outgoing arc to that package with a count of 3.
2. Edges between two files inside the selected directory are not drawn.
