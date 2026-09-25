---
id: REQ-WALK-006
uuid: 301a1ab2-8006-4d96-90ed-41fad1a4c100
title: Collisions and step height on the flat layout
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M7
verification:
  - manual
---

## Statement

The walker **shall** walk up a ledge no higher than the step height without
jumping and **shall** be stopped by anything higher, testing each axis
separately so that the walker slides along walls. The step height **shall** be
half a storey.

## Rationale

Collisions on the flat layout keep one source of truth; axis-by-axis testing
slides along walls instead of sticking to them.

## Acceptance criteria

1. Walking into a building wall stops the walker and slides them along it.
2. A ledge lower than the step height is walked up without jumping.
3. A ledge higher than the step height needs a jump or a ramp.

## Notes

Fixed: `STEP` in `walk.js` is 0.15 units, half a storey (0.3), below a terrace
wall (`TERRACE` 0.28 in `layout.js`), so a terrace needs its ramp or a jump
(`JUMP` tops out near 0.4). Ramps and bridge arches change height continuously
and stay walkable. Climbing out of the water keeps its own allowance: `WADE` rose
from 0.25 to 0.42 so that a step and a wade still clear the shore by the same
margin as before. The step up a ramp's slope in one frame can exceed 0.15 only
when running up the shortest ramp below about 23 frames per second.

Trunks and lamp posts are also obstacles (`clearProps`), which the design log
does not mention.
