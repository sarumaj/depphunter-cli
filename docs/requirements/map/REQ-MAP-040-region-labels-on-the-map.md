---
id: REQ-MAP-040
uuid: 566789ee-0b9f-4b2e-933f-699d1e782707
title: Region labels on the map
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M7
verification:
  - manual
---

## Statement

The system **shall** label regions of the map by name: islands, expanded
directories and collapsed directories, each only when its region is wide enough
on screen to be worth naming.

## Rationale

Names make the map navigable without hovering.

## Acceptance criteria

1. At the fitted view, the top-level directories and the islands are labeled.
2. Zooming out drops the labels of regions that become too small.
