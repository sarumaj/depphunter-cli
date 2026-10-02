---
id: REQ-TOOL-084
title: Grapple aim help at a distance
scope: tool
type: functional
priority: should
status: implemented
verification:
  - unit
  - manual
---

## Statement

While the grapple gun is in the off hand, a roof the view passes over within
about 4 degrees (or 0.25 units, near by) **shall** be aimed at when what the
walker is looking at would not hold the hook: the guide **shall** be drawn to
0.3 under the middle of the roof's near edge where the view crosses it, and the
hook fired there, when that edge is in reach and in plain view. Where the aim
is helped on to an edge, this one or the one REQ-TOOL-083 draws the hook up to,
two green brackets **shall** frame it on the screen.

A wall looked at past the line's reach, up to 1.6 times it, **shall** be drawn
to as a hook that would not hold, and firing **shall** say that it is out of
reach rather than aim the hook at it.

## Rationale

A roof's edge forty units off is a line a few pixels high; a look a degree too
high goes over it into the sky, and nothing at all was drawn. A wall past the
reach drew nothing either, which looked the same as aiming at nothing.

## Acceptance criteria

1. Looking just over a far roof, the guide ends on its near edge, green, with
   the brackets round it, and the hook fired there holds.
2. Looking at a wall past the reach, the guide is red, and firing says it is
   out of reach.
