---
id: REQ-CITY-037
title: Raised levels are plazas, reached by stairs
scope: city
type: functional
priority: should
status: implemented
verification:
  - unit
  - ui
---

## Statement

A terrace standing on another terrace - a level raised above the street - **shall**
be drawn as a plaza: paved in large slabs from wall to wall, with its pocket
parks as planted lawns behind a low stone edge, and no carriageway, lane
markings, manholes or crossings. A terrace standing on a plaza **shall** be
reached by a flight of stone stairs instead of a ramp: 0.8 units long with a
level landing at the top, 0.24 wide, steps at most 0.035 high, a wall with a
parapet along its open side, and no driveway; it **shall** fit a side as short
as 1.2 units. The walker **shall** go up it as up a ramp. A terrace on the
street's level keeps its ramp (REQ-CITY-017).

## Rationale

Every level of the city was a road, and every road climbed to the next on a
ramp for cars: a city of car parks. Streets belong on the ground; the levels
built up over them are for people on foot, and people on foot take the stairs.

## Acceptance criteria

1. A terrace on the street's level has streets; one raised above it is paved
   all over, with lawns where its parks are.
2. Between two raised levels the way up is a flight of stairs without a
   driveway; from a street it is a ramp with one.
3. The walker standing at the foot of a flight and walking along it ends on
   the upper level.
