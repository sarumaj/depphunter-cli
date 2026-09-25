---
id: REQ-TOOL-001
uuid: d566706c-95eb-44e5-8c2d-04af862aabd4
title: Tool held in front of the walk camera
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
  - manual
---

## Statement

Walk mode **shall** draw the walker's current tool in front of the camera as a
hand holding that tool, in a render pass of its own drawn over the world.

## Rationale

The walker uses tools, and seeing the tool in hand says which one is in use.
Drawn over the world with a cleared depth buffer, nothing in the street can cut
through the hand.

## Acceptance criteria

1. Entering walk mode shows a hand holding the current tool at the lower right
   of the view.
2. Walking against a wall never draws the wall through the hand.
