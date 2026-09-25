---
id: REQ-PERF-008
uuid: c2840360-b284-4420-bf90-af7e93274715
title: Walker ground height from a spatial grid
scope: perf
type: non-functional
priority: must
status: implemented
verification:
  - manual
  - inspection
---

## Statement

The walker **shall** determine the ground height under a point from a spatial
grid of boxes, ramps and bridge decks (2-unit cells), testing only what is
indexed in the point's cell.

## Rationale

Scanning every ramp of the map five times per step does not scale to a large
map.

## Acceptance criteria

1. Ground height queries on a large map test only the boxes, ramps and decks
   indexed in the cell under each probe.
