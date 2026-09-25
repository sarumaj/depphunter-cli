---
id: REQ-HUNT-044
uuid: 44e7641b-3ae7-479a-8512-bcf069109031
title: A shown photograph goes back down
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

A photograph shown on the camera **shall** be lowered after a few seconds (4.2
s) and the tool previously in hand **shall** be restored; a click **shall** end
the showing sooner in the same way.

## Rationale

Looking at a picture costs the walker only the seconds spent on it.

## Acceptance criteria

1. A photograph can be dismissed with a click, leaving the tool that was in hand
   before it.
2. Without a click it goes down by itself after about four seconds.
