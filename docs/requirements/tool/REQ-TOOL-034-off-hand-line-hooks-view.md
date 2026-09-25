---
id: REQ-TOOL-034
uuid: 64aa749e-b04b-4dd8-b3d2-f7caeaa2441e
title: Off-hand line goes where the view points
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

A line fired from the off hand **shall** be aimed at the building the walker is
looking at, independently of what the crosshair marks, and anything else the off
hand throws **shall** go where the view points.

## Rationale

The crosshair belongs to the primary hand.

## Acceptance criteria

1. The grapple hooks the building in the middle of the view even when the
   crosshair marks a bug.
