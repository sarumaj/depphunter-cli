---
id: REQ-WALK-039
title: Winded until a quarter is back
scope: walk
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

A walker who runs the wind gauge out **shall** be unable to run or jump until at
least a quarter of it is back.

## Rationale

Without the threshold one frame of recovery buys another stride and the gauge
flickers instead of stopping the walker.

## Acceptance criteria

1. One frame of rest after running out does not allow running or jumping.
2. After enough rest running and jumping work again.
