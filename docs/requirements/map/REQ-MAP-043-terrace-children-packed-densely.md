---
id: REQ-MAP-043
title: Terrace children packed densely
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - inspection
---

## Statement

The system **shall** pack a terrace's children, each padded by the sibling
gap, bottom-left onto a skyline, in two stable orders (tallest first, largest
first) and at a few strip widths around the side of a square of their area,
keeping the packing that takes the least area without being more than twice as
long as it is wide.

## Rationale

Terraces nest, and what one leaves empty every terrace around it carries: a
tenth left empty at each of eight levels is more than half the map. Packing
each level tightly is what keeps a deeply expanded map walkable. The stable
order keeps the same tree packing the same way.

## Acceptance criteria

1. No two children of a terrace overlap.
2. Packing the same children twice gives the same positions.
3. Large children among many small ones leave no more than a tenth of their
   terrace's packing area empty.
