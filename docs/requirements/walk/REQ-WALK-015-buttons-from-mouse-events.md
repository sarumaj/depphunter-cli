---
id: REQ-WALK-015
title: Buttons read from mouse events
scope: walk
type: constraint
priority: must
status: implemented
verification:
  - manual
  - inspection
---

## Statement

Walk mode **shall** read mouse buttons from `mousedown`/`mouseup` events, not
from pointer events.

## Rationale

Pressing a second button while one is held fires `pointermove`, not
`pointerdown`, so firing while scoped never arrived.

## Acceptance criteria

1. Holding the right button and then pressing the left button uses the tool.
