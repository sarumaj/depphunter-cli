---
id: REQ-TOOL-068
uuid: 1223d171-2f3f-4da8-82f4-dab1363496a6
title: Fishing rod holds on a wall only by chance
scope: tool
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

A fishing-rod cast that strikes a building's wall **shall** hold and pull the
walker to it at random, half of the time, wherever on the wall it strikes; the
rest of the time it **shall** glance off and return to the hand as a grapple
hook that does not hold does
([REQ-TOOL-067](REQ-TOOL-067-grapple-holds-only-near-roof.md)). The cast
**shall** still tag the module it struck either way.

## Rationale

A fish hook is not made for brick. The grapple is the tool for climbing; the
rod getting the walker about now and then is a bonus, not a way of getting
everywhere.

## Acceptance criteria

1. Over many rolls a wall hit holds in as many cases as the rod's bite chance
   says, and no more.
2. A hit at the top of a wall, where the grapple always holds, does not always
   hold with the rod.
3. A cast that does not hold still tags the module it struck.

## Notes

The chance is the rod's `reel.bite` in `web/static/tools.js`.
