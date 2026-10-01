---
id: REQ-CITY-037
title: Levels stacked above the ground are plazas, reached by stairs
scope: city
type: functional
priority: should
status: implemented
verification:
  - unit
  - ui
---

## Statement

The street's own level - a terrace standing on the land - and the blocks
standing on it are the ground, and **shall** keep their streets (REQ-CITY-006).
A terrace standing on one of those blocks or higher - a level stacked above the
ground - **shall** be drawn as a plaza: paved in large slabs from wall to wall,
with its pocket parks as planted lawns behind a low stone edge, and no
carriageway, lane markings, manholes or crossings. The way up to a plaza, and
to a block with no side long enough for a ramp, **shall** be a flight of stone
stairs instead of a ramp: 0.8 units long with a level landing at the top, 0.24
wide, steps at most 0.035 high, a wall with a parapet along its open side, and
no driveway; it **shall** fit a side as short as 1.2 units. The walker
**shall** go up it as up a ramp. A road block that has room keeps its ramp
(REQ-CITY-017).

## Rationale

Every level of the city was a road, and every road climbed to the next on a
ramp for cars: a city of car parks. Paving every level above the land instead
left almost no roads, since most of a project stands on a handful of top-level
blocks. Streets belong on the ground, the land and the blocks on it; the levels
built up over them are for people on foot, and people on foot take the stairs.

## Acceptance criteria

1. A terrace on the land and a block standing on it have streets; a level on
   that block or higher is paved all over, with lawns where its parks are.
2. The way up to a plaza is a flight of stairs without a driveway; up to a
   road block it is a ramp with one, or stairs when no side has room for a
   ramp.
3. The walker standing at the foot of a flight and walking along it ends on
   the upper level.
