---
id: REQ-WALK-054
title: The view banks in flight
scope: walk
type: functional
priority: should
status: implemented
verification:
  - unit
  - manual
---

## Statement

While flying, the walk camera **shall** roll about its line of sight into a
turn, in proportion to how fast the view is turning (by keys or by the mouse),
and into a sideways move, by at most 0.42 radians (about 24 degrees) either
way, easing into the bank and back out of it. On the ground, during the way in,
and where the page asks for reduced motion, it **shall** stay level. Rolling
**shall not** change where the view points.

## Rationale

Anything that turns in the air leans into the turn; a view that swings round
level reads as a camera on a tripod rather than a flight. Only pitch and yaw are
steered, so aiming, the tools and everything that reads the view's direction
work as before.

## Acceptance criteria

1. A held left turn in flight settles on a lean to the left, a right turn to
   the right, and a harder turn leans further, up to the limit.
2. Turning on the spot on foot does not lean the view.
3. The lean eases in over several frames and is gone shortly after landing.
