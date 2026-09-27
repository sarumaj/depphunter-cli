---
id: REQ-WALK-022
title: No hover card while the walker is held
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

While the walker is held, the crosshair **shall not** be recomputed and the
hover card and highlight of whatever it was last on **shall** be cleared.

## Rationale

A card left on top of what is being read is worse than no card.

## Acceptance criteria

1. Opening a building's details in walk mode leaves no hover card on the panel.
