---
id: REQ-HUNT-040
title: Photographs leave out selection and arcs
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

A photograph **shall not** contain selection highlighting, selection dimming,
the selection outline or dependency arcs, and the view **shall** be restored
afterwards.

## Rationale

A module tagged a minute ago should not be lit in a picture of the street it
stands on.

## Acceptance criteria

1. A photograph taken with a module selected shows no arcs, outline, highlight
   or dimming.
2. The selection is still shown on screen after the photograph.
