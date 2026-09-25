---
id: REQ-PERF-003
uuid: fe6756de-a479-4614-b19a-408f83c13770
title: Labels throttled while walking
scope: perf
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - manual
  - inspection
---

## Statement

While walking, the UI **shall** lay out the labels at most once every 90 ms, for
frames drawn by both the map's renderer and the walker's own loop, through one
shared throttle.

## Rationale

Labels are placed greedily over all boxes; laying them out every frame costs a
frame on large repositories.

## Acceptance criteria

1. In walk mode, label layout runs a few times a second rather than on every
   frame.
