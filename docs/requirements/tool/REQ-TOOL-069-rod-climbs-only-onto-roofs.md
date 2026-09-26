---
id: REQ-TOOL-069
uuid: ec0cf988-6354-408f-9251-8fc2d6ac4b83
title: Fishing rod climbs only from a cast onto a roof
scope: tool
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

A fishing-rod cast that comes down on a building's roof **shall** hold and wind
the walker up onto that roof, as the grapple does, more slowly and on a line of
at most 28 map units. A cast that strikes a building's wall, at any height,
**shall not** hold: it **shall** glance off and return to the hand as a grapple
hook that does not hold does
([REQ-TOOL-067](REQ-TOOL-067-grapple-holds-only-near-roof.md)). The same cast
**shall** always have the same outcome, and **shall** tag the module it struck
either way.

## Rationale

A fish hook is not made for brick, so the rod asks for a clean shot at the roof
where the grapple forgives two storeys of facade. In return it climbs from the
hunting hand, which leaves the off hand free for the jet backpack or the water
skimmers. A rule the walker can learn beats a chance they cannot.

## Acceptance criteria

1. A cast onto a roof holds, every time, and sets the walker on the roof.
2. A cast at a wall, the top of it included, never holds.
3. The rod's line is shorter and slower than the grapple's.
4. A cast that does not hold still tags the module it struck.

## Notes

The rule is the rod's `reel.roof` in `web/static/tools.js`.
