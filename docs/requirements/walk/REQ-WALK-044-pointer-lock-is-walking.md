---
id: REQ-WALK-044
uuid: 928bce46-7175-4e1a-975b-858caa936977
title: Pointer lock state decides walking
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M33
verification:
  - manual
---

## Statement

Where pointer lock can be obtained, losing the lock **shall** hold the walker
and regaining it **shall** resume walking; this **shall** be decided from the
lock state reported by the browser (`pointerlockchange`), not by counting
requests.

## Rationale

Letting the pointer go used to stop nothing: the walker went on walking on
whatever key was down, spending wind and drowning behind a dropdown. Both halves
are needed, or a walker is left captured and held.

## Acceptance criteria

1. `Esc` in the street holds the walker where they stand.
2. Regaining the lock by any means resumes walking.
3. Several overlapping lock requests leave the walker in a consistent state.
