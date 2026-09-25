---
id: REQ-CITY-021
uuid: b7b5b335-47ee-4be5-89e7-47177f482411
title: Ramps computed once per layout
scope: city
type: non-functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The ramps and bridges of a layout **shall** be computed once per boxes array,
cached, and shared by the renderer and the walker's collisions.

## Rationale

Decision: one computation guarantees that what is drawn is what is walked
on, and avoids recomputing it per frame.

## Acceptance criteria

1. `rampsFor` and `bridgesFor` return the identical array for the same boxes
   array.
