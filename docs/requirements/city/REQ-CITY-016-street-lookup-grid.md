---
id: REQ-CITY-016
title: Street lookup grid
scope: city
type: non-functional
priority: must
status: implemented
verification:
  - manual
  - inspection
---

## Statement

For street shading the system **shall** build a lookup grid of 0.5-unit cells
(coarser on huge maps, at most 2^20 cells and 4096 texels a side) that lists, in
a float texture, the 8 footprints nearest to each cell within 0.9 units, beside
a texture of all footprints; the grid **shall** be rebuilt after every relayout.

## Rationale

Decision: the shader measures exact distances to a handful of nearby
obstacles instead of looping over all boxes. Footprints a fragment lies inside
are skipped, so one 2D grid serves every terrace level.

## Acceptance criteria

1. A map of 10 000 files renders streets without a frame-rate drop.
2. After a relayout the streets follow the new layout.

## Notes

The isometric map draws streets too, so the grid is built for every layout
(`MapScene.setBoxes`), not only when walk mode is shown.
