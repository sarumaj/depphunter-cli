---
id: REQ-PERF-004
uuid: 24dae790-f0c2-4fb3-9c3f-bb7218638857
title: Walk-mode ground recolored only while walking
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

The system **shall** recolor the walk-mode ground buffer only while walk mode is
shown, deferring outside it to one full repaint on entering walk mode.

## Rationale

The ground buffer holds hundreds of thousands of values and is not drawn on the
isometric map.

## Acceptance criteria

1. Hovering buildings on the isometric map does not write the ground buffer.
2. Entering walk mode shows the ground in the current colors.
