---
id: REQ-WALK-009
title: Planet radius bounds
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

The planet radius **shall** be at most three times the diagonal of the layout
(bounded to 60 to 2000 units) and at least 6 units, and on entering walk mode
**shall** start at 0.6 times the diagonal within those bounds (and at most 600).
A change of radius from the keys **shall** be animated: the curvature eases to
the new radius, in proportion rather than in units, most of the way in about a
third of a second, while the HUD shows the new radius at once. With
`prefers-reduced-motion: reduce` it **shall** change at once.

## Rationale

The planet can grow until the map looks flat, not beyond. A curvature that
changed in one frame threw the whole horizon at the walker at once.

## Acceptance criteria

1. Repeatedly growing the planet stops at three times the map diagonal.
2. Repeatedly shrinking it stops at a radius of 6.
3. Pressing `]` bends the horizon over a few frames, not in one, and it settles
   on the new radius.
