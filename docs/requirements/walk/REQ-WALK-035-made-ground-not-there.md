---
id: REQ-WALK-035
uuid: 8cea30c4-db81-49e8-be7e-794e0db44396
title: Ground off the end of a deck is not ground
scope: walk
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The height of a ramp or bridge deck **shall** be undefined (`-Infinity`) outside
its footprint, and the ground height beside a bridge **shall** be what is
actually there, so no cell of open water reads as walkable ground.

## Rationale

Marking the cells off the end of a deck dry made the open water beside a bridge
walkable at the causeway's level.

## Acceptance criteria

1. `bridgeHeight` beside the deck returns `-Infinity`.
2. Stepping off the side of a bridge (by jumping) lands in water, not on an
   invisible causeway.
