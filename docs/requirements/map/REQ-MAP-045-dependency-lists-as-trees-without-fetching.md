---
id: REQ-MAP-045
uuid: 310b9324-65c7-4206-87c1-3e94d916c41a
title: Dependency lists as trees without fetching
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The side panel **shall** present dependencies and dependents as trees: a row
with further dependencies (in the same direction) **shall** open into them,
built from the edges already in the model, without any request to the server.

## Rationale

A supply chain is a tree once transitive packages are on the map, and every edge
already arrived with the graph.

## Acceptance criteria

1. Opening a package row that has dependencies lists them indented below it.
2. Opening a row makes no network request.
