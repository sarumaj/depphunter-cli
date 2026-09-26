---
id: REQ-TOOL-067
uuid: 768285e0-b4ab-48c4-ace9-b0871e1ed312
title: Grapple holds only near a roof's edge
scope: tool
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

A grapple hook that strikes a building **shall** hold, every time, where it
strikes within 0.6 map units (two storeys) of the building's roof edge, and
**shall not** hold anywhere lower; a hook that strikes the ground or a terrace
**shall** hold as before. A hook that does not hold **shall** pull nobody: it
**shall** be seen to glance off the face it struck, with a puff at the point of
impact, tumble and drop under gravity until it lands, and then be reeled back
to the hand along its line.

## Rationale

A claw closes on a parapet, not on a flat wall; aiming at the top of a facade
is what climbing it takes. A miss that simply vanished would look like a bug
rather than a miss.

## Acceptance criteria

1. A hook at a roof's edge, or within 0.6 units below it, holds whatever the
   roll of the dice.
2. A hook at the foot of a wall, or more than 0.6 units below the edge, does not
   hold.
3. A hook in the street holds, so a shot off a roof is still the way down.
4. A hook that does not hold rebounds off the face it struck and returns to the
   hand; it tags nothing and holds on nothing on the way.

## Notes

The grip is the grapple's `reel.grip` in `web/static/tools.js`.
