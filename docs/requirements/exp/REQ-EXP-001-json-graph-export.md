---
id: REQ-EXP-001
uuid: c5b9e397-6f6b-4313-ad71-4c2ff121a307
title: JSON graph export
scope: exp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M3
verification:
  - unit
---

## Statement

The system **shall** export the graph as JSON in the format of the graph
document the UI reads.

## Rationale

JSON is the lossless format for other tools and for re-reading.

## Acceptance criteria

1. A JSON export decodes back into a graph with the same number of nodes and
   edges.
