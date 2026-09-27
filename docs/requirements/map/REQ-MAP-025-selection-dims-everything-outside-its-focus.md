---
id: REQ-MAP-025
title: Selection dims everything outside its focus
scope: map
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

When a node is selected, the system **shall** draw every box in the dim color,
without facade detail, except the selection's representative, its descendants,
the ends of its drawn arcs, and structural boxes (land and terraces).

## Rationale

Dimming makes the neighborhood of the selection stand out without hiding the
rest of the map.

## Acceptance criteria

1. Selecting a file leaves it and the boxes its arcs reach in their colors and
   dims all other buildings.
2. Clearing the selection restores every color.
