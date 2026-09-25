---
id: REQ-WALK-006
uuid: 301a1ab2-8006-4d96-90ed-41fad1a4c100
title: Collisions and step height on the flat layout
scope: walk
type: functional
priority: must
status: partial
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

Partial: `STEP` in `walk.js` is 0.32 units, more than a storey (0.3) and more
than a terrace wall (`TERRACE` 0.28 in `layout.js`), so terrace walls are walked
up without a jump or a ramp (the stairs shader comment says so). The design log
says "half a storey". Trunks and lamp posts are also obstacles (`clearProps`),
which the design log does not mention.
