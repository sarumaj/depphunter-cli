---
id: REQ-CITY-017
uuid: 83bef331-4763-4c69-84d4-34d2dae350d5
title: A ramp for every nested terrace
scope: city
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

Every nested terrace (a city block, not a file's symbol plot) with a side at
least 1.7 units long **shall** get one ramp, 0.2 units wide, along the side with
the most room beside it, in the street beside it, rising from a corner at the
street's level to the terrace's top over at most 2.4 units.

## Rationale

Decision: streets are 0.35-0.55 units wide, and climbing a 0.28-unit
terrace across that distance would be a wall, not a road; a ramp along the side
has room to be a road.

## Acceptance criteria

1. A walker can drive up every nested block by its ramp from one street onto the
   other without crossing a curb.
