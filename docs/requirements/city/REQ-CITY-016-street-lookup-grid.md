---
id: REQ-CITY-016
uuid: 65800e94-b9c0-4822-9f06-bfce650d8f8d
title: Street lookup grid
scope: city
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M8
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

Decision (M8): the shader measures exact distances to a handful of nearby
obstacles instead of looping over all boxes. Footprints a fragment lies inside
are skipped, so one 2D grid serves every terrace level.

## Acceptance criteria

1. A map of 10 000 files renders streets without a frame-rate drop.
2. After a relayout the streets follow the new layout.

## Notes

The design log says the grid is built only when walk mode is shown. Since M10
the isometric map draws streets too, so the grid is built for every layout
(`MapScene.setBoxes`).
