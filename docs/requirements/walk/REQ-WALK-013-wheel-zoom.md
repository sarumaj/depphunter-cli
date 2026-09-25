---
id: REQ-WALK-013
uuid: 5110d139-a813-4e1c-87a6-c6cbcb052120
title: Wheel zooms the walk view
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - manual
---

## Statement

In walk mode the mouse wheel **shall** change the field of view between 30° and
90°.

## Rationale

First-person-shooter habit; the planet radius has its own keys.

## Acceptance criteria

1. Scrolling narrows or widens the view and stops at 30° and 90°.
2. The wheel does nothing while the walker is held (REQ-WALK-021).
