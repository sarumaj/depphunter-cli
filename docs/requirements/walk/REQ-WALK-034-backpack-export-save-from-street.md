---
id: REQ-WALK-034
uuid: f3651e30-8348-463d-a034-19bc2f6342f1
title: Backpack, export and save from the street
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M21
verification:
  - manual
---

## Statement

In walk mode `B`, `X` and `K` **shall** open the backpack, open the export menu
and save the view respectively; opening the backpack or the export menu
**shall** release the pointer and hold the walker (REQ-WALK-021), and closing
either **shall** resume walking.

## Rationale

The toolbar is behind a captured pointer while walking, so these need keys.

## Acceptance criteria

1. `B` in the street opens the backpack with a visible cursor.
2. `X` opens the export menu the first time it is pressed.
3. Closing either returns to the reticle.
