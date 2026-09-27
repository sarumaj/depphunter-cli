---
id: REQ-MAP-021
title: Zooming out stops at 35 percent
scope: map
type: functional
priority: must
status: implemented
verification:
  - manual
  - e2e
---

## Statement

The system **shall** stop zooming out of the isometric view when the whole map
fills 35% of the view.

## Rationale

Beyond that point the map becomes a speck and there is nothing left to navigate.

## Acceptance criteria

1. Zooming out as far as possible leaves the map filling 35% of the view's width
   or height.
2. The limit is recomputed when the window is resized or the map is laid out
   again.
