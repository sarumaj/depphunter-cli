---
id: REQ-WALK-064
title: The walker, the balls and the bugs cast shadows
scope: walk
type: functional
priority: may
status: implemented
verification:
  - unit
  - manual
---

## Statement

In walk mode, the walker, every ball on a court and every bug near the walker
**shall** have a shadow on what is under them, drawn as the props' are
(REQ-CITY-042):

- the walker's, while their body is drawn, on what they stand on - the street,
  a roof, a ramp, lying along its slope - and, in the air, on what is straight
  under them; not while swimming or going under;
- a ball's under it wherever it rolls, flies or is carried;
- a bug's on whatever it walks - a wall as well as a roof or the street - and a
  flying bug's on what is under it; none for a bug caught;
- each the wider and the fainter the higher above the ground its caster is,
  and gone well above it, so that a jump, a throw or a flight shows how high it
  went and where it will come down.

Out of walk mode, none of them **shall** be drawn.

## Rationale

Without a shadow, something in the air cannot be told from something further
off: a jump or a ball's arc reads as sliding about on the screen, and nothing
says where it will land.

## Acceptance criteria

1. Standing, the walker has a soft shadow at their feet; jumping, it stays on
   the ground, fainter and wider at the top of the jump.
2. On a ramp, the walker's shadow lies along the ramp.
3. A ball thrown high has a faint shadow under it that darkens as it comes
   down.
4. A beetle on a wall has its shadow on the wall; a caught bug has none.
