---
id: REQ-WALK-046
uuid: 7c94ea77-82d2-4550-95f1-c0bab2baf950
title: Page keeps its keys while the walker is held
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

While the walker is held, walk mode **shall** take only `V`, `M`, `Esc` and
`Enter`, and **shall not** take them when the event comes from a focused control
(other than the Walk button) where they belong to the control.

## Rationale

Swallowing every key made a toolbar button impossible to work by keyboard once
the pointer had been let go.

## Acceptance criteria

1. After `Esc`, every toolbar control can be worked by mouse and by keyboard
   without the reticle taking the mouse back.
2. `Enter` on a focused toolbar button activates the button.
