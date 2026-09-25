---
id: REQ-TOOL-003
uuid: 12e4bc58-73d3-4f9a-ab63-6511a1f10ad6
title: Tool choice saved with the view
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
  - inspection
---

## Statement

The primary tool in hand **shall** be saved with the rest of the view as the
setting `ui.tool`, and walk mode **shall** start with the saved tool.

## Rationale

A walker's preferred tool is part of how they view the map.

## Acceptance criteria

1. After choosing the camera and saving the view, a reload enters walk mode with
   the camera.
2. A `ui.tool` value that names no tool is rejected by the configuration
   validation.
