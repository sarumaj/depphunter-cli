---
id: REQ-HUNT-046
uuid: d7d55206-bc85-4628-a1e3-a2d42a424ab3
title: Photographs fitted to the screen by cropping
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M27
verification:
  - ui
---

## Statement

A photograph shown on the camera screen **shall** fill the screen at its own
aspect ratio, cropping the long side, and **shall not** be squashed or tiled.

## Rationale

The screen is squarer than the window the picture was taken through.

## Acceptance criteria

1. For 16:9, 9:16 and near-screen-shaped pictures, the shown region has the
   screen's aspect ratio.
2. The texture repeat never exceeds 1 on either axis.
