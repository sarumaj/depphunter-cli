---
id: REQ-TOOL-002
uuid: d644d0c4-bc46-4194-9895-635ee340545b
title: Animated gesture for using a tool
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - e2e
  - manual
---

## Statement

Using a tool **shall** play an animated gesture specific to that tool: the rod
casts, the net sweeps, the camera's shutter kicks back, the bubble wand waves
and the dart gun recoils.

## Rationale

The gesture of using a tool is animated rather than implied.

## Acceptance criteria

1. Each tool plays its gesture on use and returns to its rest pose afterwards.
2. The gesture runs to its end even if the shot lands sooner.
