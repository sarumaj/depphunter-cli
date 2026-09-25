---
id: REQ-TOOL-061
uuid: 1ca0668e-db15-4542-9223-3a05b7c905a6
title: Esc and right button leave the wheel
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

`Esc` and the right mouse button **shall** close the wheel without changing what
is in either hand.

## Rationale

A wheel opened by accident should cost nothing.

## Acceptance criteria

1. Opening the wheel, pointing at a tool and pressing `Esc` leaves both hands
   unchanged.
