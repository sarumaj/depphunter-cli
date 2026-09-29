---
id: REQ-TOOL-075
title: A landing under a canopy costs its speed
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

Under a canopy the ground **shall** cost what arriving at the touchdown speed
costs - the sink and 0.4 of the speed along the ground - rather than the height
fallen: nothing up to 1.2 units a second (about four meters a second), and a
share of the walker growing with the speed from there, all of it at the speed a
lethal fall arrives at. Flying into a wall **shall** cost the same for the speed
into it. Landing in the water **shall** cost nothing.

## Rationale

What hurts in a canopy landing is the speed, and the flare is the way to have
less of it. The height the canopy was flown down from has nothing to do with it.

## Acceptance criteria

1. A well-flared landing costs nothing.
2. An unflared landing costs between two and fifteen hundredths of the walker,
   one on full flight more, and neither depends on the height flown from.
3. A wall flown into at glide speed costs more than a tenth of the walker.
4. A landing at the speed of a lethal fall is lethal.

## Notes

The law is `Health.touchdown` in `web/static/health.js`.
