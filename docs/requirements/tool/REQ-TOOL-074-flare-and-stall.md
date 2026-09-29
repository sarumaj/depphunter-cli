---
id: REQ-TOOL-074
title: Space flares the canopy, and too early stalls it
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

Under an open canopy `Space` **shall** flare it: for a little over a second the
canopy trades forward speed for lift, holding the sink near nothing. A flare
spent before the ground **shall** leave the canopy stalled for a moment,
sinking faster than it would have without the flare.

## Rationale

The flare is what makes the landing a thing to get right, and a flare that
could be held from any height would be no skill at all.

## Acceptance criteria

1. A flare begun about two meters up comes in at under two thirds of the speed
   of none.
2. A flare begun five meters up stalls and comes in sinking faster than none.
