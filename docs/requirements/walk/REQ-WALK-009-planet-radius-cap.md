---
id: REQ-WALK-009
uuid: 24182103-3831-4fbb-8f36-17c7b6f22f7d
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

## Rationale

The planet can grow until the map looks flat, not beyond.

## Acceptance criteria

1. Repeatedly growing the planet stops at three times the map diagonal.
2. Repeatedly shrinking it stops at a radius of 6.
