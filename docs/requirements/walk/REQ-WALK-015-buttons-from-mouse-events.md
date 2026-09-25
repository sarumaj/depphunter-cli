---
id: REQ-WALK-015
uuid: 9f6b2863-9ad2-4014-8bcf-9a98e9b80235
title: Buttons read from mouse events
scope: walk
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
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
