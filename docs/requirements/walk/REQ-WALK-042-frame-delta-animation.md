---
id: REQ-WALK-042
uuid: 4b4ddc56-4243-4d2e-9b95-ebb7cae67cc4
title: Animation advanced by frame delta
scope: walk
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M27
verification:
  - ui
  - inspection
---

## Statement

Any motion whose rate can change **shall** be advanced by the frame's own time
step rather than computed as elapsed time multiplied by a rate, and easing
**shall** be expressed over seconds rather than over frames.

## Rationale

A phase written as elapsed time times a rate jumps by hundreds of radians when
the rate changes an hour into a session; easing per frame settles faster on a
faster screen.

## Acceptance criteria

1. The held tool does not jerk as the walker breaks into a run an hour into a
   session.
2. The scope's field-of-view easing takes the same time at 30 and at 144 frames
   per second.
