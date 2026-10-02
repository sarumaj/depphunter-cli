---
id: REQ-TOOL-083
title: Aiming help - lock-on for bugs, roof edges for the grapple
scope: tool
type: functional
priority: should
status: implemented
verification:
  - unit
  - manual
---

## Statement

For a tool that throws at bugs, a bug the predicted path (REQ-TOOL-082) passes
within 0.45 units plus 3.5% of its distance **shall** be locked on to: the shot
**shall** turn on to it as it flies - by a share of the way for every unit
flown, so a fast shot turns as surely as a slow one - and follow it if it walks
on; the guide **shall** be flown the same way and end on it, and four corners
**shall** frame it on the screen. A bug the shot would still not reach, homing,
**shall not** be locked on to.

While the grapple gun is in the off hand, walk mode **shall** draw where its
hook would bite, as the trajectory guide does, colored by whether the hook
would hold there or glance off. A hook that would bite too low on a wall to
hold **shall** be drawn up the wall to 0.3 under its roof's edge, and fired
there, when that edge is in reach, in plain view and within about 13 degrees of
where the walker is looking.

## Rationale

Aimed by a guide rather than a crosshair, a beetle a few pixels across at the
far side of a street is hard to put a shot on, and harder while it walks; a
lock-on that the guide shows, and the shot honours, makes it a matter of
pointing near it. A grapple only holds near the top of a wall, and nothing said
where that was until the hook had glanced off.

## Acceptance criteria

1. A nail aimed just off a bug locks on to it, the guide ends on it, and the
   nail fired catches it.
2. A bug well off the line is not locked on to.
3. Looking at a wall a little under its roof, the grapple's guide goes up to
   the roof's edge and shows the hook holding; looking at the foot of a tall
   wall close by, it shows the hook glancing off.
