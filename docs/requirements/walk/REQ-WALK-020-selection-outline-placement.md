---
id: REQ-WALK-020
uuid: 43972229-9fc0-4199-b504-e452bba7e5ff
title: Selection outline placement and occlusion
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - manual
---

## Statement

The selection outline **shall** enclose its box from base to top. On the
isometric map it **shall** be drawn through whatever hides it; in walk mode it
**shall** be hidden behind nearer geometry.

## Rationale

It floated half a box too high since M7. In walk mode, big blocks surround the
walker and an outline through them is noise.

## Acceptance criteria

1. The outline of a selected building meets the ground.
2. In walk mode a selected building behind another shows no outline through it.
