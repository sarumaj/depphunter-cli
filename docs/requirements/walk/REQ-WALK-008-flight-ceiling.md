---
id: REQ-WALK-008
uuid: b3cba739-aab2-495c-8ae3-5bc5d032a2fc
title: Flight ceiling above the tallest box
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M8
verification:
  - manual
---

## Statement

A flying walker's feet **shall not** rise more than 12 units above the top of
the tallest box of the layout.

## Rationale

Flying beyond that shows nothing more of the map and loses it behind the
planet's curve.

## Acceptance criteria

1. Flying straight up stops 12 units above the tallest building.
