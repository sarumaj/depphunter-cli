---
id: REQ-PERF-006
uuid: 2d0b307d-8234-4b17-866d-b0cab2b7c9d4
title: Recoloring touches only changed boxes
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

The system **shall** write instance colors only for boxes whose color or fade
flag changed since the last recoloring, and **shall** repaint and upload only
those boxes' vertex ranges of the walk-mode ground.

## Rationale

Pointing at a building recolors two boxes; repainting the whole ground for that
took longer than a frame.

## Acceptance criteria

1. Hovering a building in walk mode uploads only that building's vertex ranges
   of the ground buffer.
2. A recoloring that changes nothing uploads nothing.
