---
id: REQ-WALK-011
uuid: c726d371-c5f5-4233-93ee-40b42e72088d
title: Mouse look and pointer glitch filter
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

With the pointer locked, mouse movement **shall** turn the view (0.0022 radians
per pixel at the default field of view, pitch limited to ±1.5 radians). The
first movement event after the lock is gained **shall** be ignored, and a
movement of 250 px or more on either axis **shall** be limited to 250 px.

## Rationale

Some browsers report a bogus jump with the first event after locking or after a
focus change; without the filter the view snaps to a random direction.

## Acceptance criteria

1. Locking the pointer does not turn the view by itself.
2. A single synthetic movement of 1000 px turns the view no more than one of 250
   px.

## Notes

Large jumps are clamped rather than ignored so that a fast flick still turns the
view.
