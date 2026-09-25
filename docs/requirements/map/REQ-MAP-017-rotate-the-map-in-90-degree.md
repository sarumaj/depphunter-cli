---
id: REQ-MAP-017
uuid: 809c0b21-3b94-45db-9239-c744be3953cc
title: Rotate the map in 90-degree steps
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

The UI **shall** rotate the isometric view by 90 degrees to the left and right,
from toolbar buttons and the keys `Q` and `E`, restoring the isometric
elevation.

## Rationale

Quarter turns reveal what taller buildings hide without losing the isometric
view.

## Acceptance criteria

1. Pressing `E` four times returns the view to its starting orientation.
2. The rotate buttons do the same as `Q` and `E`.
