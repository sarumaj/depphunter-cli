---
id: REQ-PERF-013
title: The boxes' depth drawn before their color
scope: perf
type: non-functional
priority: should
status: implemented
verification:
  - ui
  - manual
---

## Statement

Each frame **shall** first draw the depth of the boxes and of the setback
towers' tiers alone, writing no color, so that the city's shader then runs for
the nearest surface of each pixel only; the frame's own drawing of a surface
**shall** never be hidden by that surface's depth from the pre-pass.

## Rationale

The boxes are drawn in the layout's order, not nearest first, and walking
along a street a pixel is covered by two boxes on average. The city's shader
is costly per pixel, and a GPU skips it for a pixel whose depth already lies
in front.

## Acceptance criteria

1. The pre-pass draws only the meshes on the boxes' layer, with no color
   written, pushed slightly back so the frame's own drawing passes the depth
   test.
2. The camera's layers and the scene are as they were after the pre-pass, and
   the frame does not clear the depth it wrote.
