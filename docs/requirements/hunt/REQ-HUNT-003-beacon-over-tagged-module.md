---
id: REQ-HUNT-003
uuid: 51d5b198-dac4-41c9-a01f-fdf581392ee8
title: Beacon over every tagged module
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
  - manual
---

## Statement

The system **shall** mark every tagged module that has a box on the map with a
beacon, consisting of a thin vertical beam and a diamond above the module, for
the rest of the session.

## Rationale

The hunt's trophies have to be visible across the city so the walker can see
where they have already been.

## Acceptance criteria

1. After a module is tagged, a beam and a diamond stand above its box.
2. The beacon remains after a relayout for as long as the module has a box.
3. The beacon is drawn with a bendable material and follows the planet's curve.
