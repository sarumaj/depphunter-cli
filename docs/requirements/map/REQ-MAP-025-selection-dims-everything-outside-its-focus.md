---
id: REQ-MAP-025
uuid: 3746bbec-26f6-4eca-a31e-31a3f84dc93f
title: Selection dims everything outside its focus
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M1
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
