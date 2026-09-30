---
id: REQ-PERF-010
title: Props culled by view and drawn coarse when small
scope: perf
type: non-functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The props of a map (trees, bushes, lamps and their glow) **shall** be drawn only
for the cells of the map the camera sees, and while walking only for those
within the walker's horizon and the fog; a prop that spans only a few pixels on
screen **shall** be drawn with a simplified copy of its model, and each kind of
prop **shall** still take one draw for each level of detail.

## Rationale

A map holds thousands of props, most of them behind the walker, beyond the
horizon, or a pixel wide on a map zoomed out. Drawing each one of them in full
on every frame costs more than the buildings do.

## Acceptance criteria

1. A cell of props out of the map camera's view is not drawn.
2. A cell past the walker's horizon is not drawn.
3. As props shrink on screen they are drawn with coarser copies of their
   models, each with fewer triangles than the one before.
4. Before any frame has been judged, every prop is drawn in full.
