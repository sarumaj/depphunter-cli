---
id: REQ-WALK-025
title: Relayout keeps the walker in place
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

After a relayout (depth change, filter, live update) the walker **shall** be put
back at the same distance from the same edge of the block they stood on, and
**shall** step aside to the nearest spot no higher than their level if a
building now occupies it.

## Rationale

A relayout should rebuild the city around the walker instead of teleporting
them.

## Acceptance criteria

1. A live update while walking leaves the walker beside the same block.
2. A walker whose spot is taken by a new building is moved to a free spot
   nearby.
