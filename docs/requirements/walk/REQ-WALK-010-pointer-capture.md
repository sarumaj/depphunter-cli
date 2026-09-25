---
id: REQ-WALK-010
uuid: b0a68046-83bb-4d9d-902e-16ac8a1c6767
title: Pointer capture on entering walk mode
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
  - e2e
---

## Statement

Entering walk mode **shall** request pointer lock on the map canvas, unless
something on screen wants the pointer. `Esc` **shall** release the lock, and a
click on the map **shall** request it again.

## Rationale

First-person-shooter habit: the mouse looks around, and a reticle fixed in the
centre aims. `Esc` must hand the pointer back so the toolbar over the street can
be reached.

## Acceptance criteria

1. After entering walk mode the cursor is hidden and mouse movement turns the
   view.
2. `Esc` shows the cursor again.
3. A click on the map captures the pointer again.
