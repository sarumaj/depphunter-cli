---
id: REQ-WALK-004
uuid: 4e187fc5-1b3a-4102-921d-28aaa9064553
title: Walking movement keys
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M7
verification:
  - manual
  - e2e
---

## Statement

In walk mode `W`/`ArrowUp` **shall** move the walker forward, `S`/`ArrowDown`
backward, `A` and `D` **shall** strafe left and right, `ArrowLeft` and
`ArrowRight` **shall** turn the walker, and holding `Shift` **shall** make the
walker run (2.6 units/s walking, 7 units/s running).

## Rationale

First-person conventions that any player already knows.

## Acceptance criteria

1. Each key moves or turns the walker as stated.
2. With `Shift` held the walker covers ground faster than without.
3. Running is refused while the walker is winded (REQ-WALK-039).
