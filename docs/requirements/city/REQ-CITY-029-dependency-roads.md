---
id: REQ-CITY-029
uuid: e8091f31-f101-433b-b64e-4d22760fa658
title: Dependency roads
scope: city
type: functional
priority: must
status: withdrawn
source:
  - docs/REQUIREMENTS.md M20
  - docs/REQUIREMENTS.md M23
  - docs/REQUIREMENTS.md M24
verification:
  - manual
---

## Statement

The system **shall** draw a road on the ground for each dependency between two
selected buildings, routed over a grid that takes ramps and bridges as ground.

## Rationale

The routes were meant to show how dependencies connect across the city.

## Acceptance criteria

1. Withdrawn; no longer verified.

## Notes

Added in M20 and reworked in M23 (settling every reachable cell, carrying roads
wall to wall). Withdrawn in M24: a route found on a grid over a city not laid
out for one went missing where the sweep could not reach and climbed walls where
no ramp had been fitted. The arcs say which buildings are joined.
