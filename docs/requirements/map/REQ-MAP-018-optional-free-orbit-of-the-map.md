---
id: REQ-MAP-018
uuid: 3fe0201b-d551-4643-a026-616306f0a061
title: Optional free orbit of the map view
scope: map
type: functional
priority: may
status: implemented
source:
  - docs/REQUIREMENTS.md §5
verification:
  - manual
---

## Statement

The UI **may** orbit the map view freely on a right-button drag, between polar
angles of 0.15 and 1.35 radians.

## Rationale

A free orbit helps to look into narrow streets; it is bounded so the view never
goes under the map.

## Acceptance criteria

1. Dragging with the right button orbits the view, and it cannot be turned to
   look from below the ground.
