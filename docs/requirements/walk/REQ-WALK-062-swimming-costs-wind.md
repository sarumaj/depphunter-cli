---
id: REQ-WALK-062
title: Swimming and the swim ring cost wind
scope: walk
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

Swimming in the swim ring **shall** be paid for out of the walker's wind
(REQ-WALK-038) rather than a tank: keeping afloat at 0.3 of what running costs,
swimming at 0.8 of it and swimming flat out at 1.5 times it. Walking with the
ring on out of the water **shall** cost 0.25 of what running does, and running
with it 1.5 times. A swimmer **shall** be told to make for the shore when their
wind falls to 30 %, and a swimmer out of breath **shall** go under until they
have it back (REQ-TOOL-049).

## Rationale

Swimming is harder work than walking, and the ring is clumsy to walk in; one
gauge for what the walker's own body has left keeps that legible, where a tank
for a ring said it was the ring that tired.

## Acceptance criteria

1. The swim ring has no tank and shows no fuel gauge.
2. Swimming drains the wind, faster flat out and slower keeping still; walking
   ashore without the ring does not.
3. Walking with the ring on drains the wind; running with it drains it faster
   than running without it.
4. Out of breath, the ring stops holding the walker up.
