---
id: REQ-MAP-016
uuid: 64e5ad1f-c2fa-439b-ac71-aaa5e0af96e0
title: Pan and zoom the map view
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
verification:
  - manual
  - e2e
---

## Statement

The UI **shall** pan the map view on a left-button drag (or a one-finger drag)
and zoom it towards the cursor with the mouse wheel (or a two-finger pinch).

## Rationale

Pan and zoom are the basic ways to navigate a map larger than the screen.

## Acceptance criteria

1. Dragging with the left button moves the map with the pointer.
2. Turning the wheel zooms in and out about the point under the cursor.
