---
id: REQ-WALK-012
uuid: 1aa48709-461c-42df-9839-77a0d099691e
title: Scope on the right mouse button
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

Holding the right mouse button in walk mode **shall** ease the field of view
from its current value (70° by default) to 22°, releasing it **shall** ease it
back, and the look sensitivity **shall** scale with the field of view.

## Rationale

A scope makes distant buildings and bugs hittable; scaled sensitivity keeps the
reticle steady through it.

## Acceptance criteria

1. Holding the right button zooms in smoothly to a 22° view.
2. Mouse movement turns the scoped view proportionally less.
3. Releasing the button returns to the previous field of view.
