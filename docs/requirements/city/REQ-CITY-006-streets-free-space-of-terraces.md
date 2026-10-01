---
id: REQ-CITY-006
title: Streets as the free space of terrace tops
scope: city
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

The top of every terrace on the ground - one standing on the land, and a block
standing on one of those - **shall** be drawn as streets wherever it is not
covered by a child box: gaps between children, 0.7 units, **shall** be side
streets and the padding along the terrace edge, 1.1 units, a ring road, so
streets connect by construction.
A terrace stacked higher is a plaza instead (REQ-CITY-037).

## Rationale

Deriving streets from free space needs no routing and cannot leave a building
unconnected. The gaps were half as wide, which left a street narrower than two
walkers side by side once its sidewalks were taken off.

## Acceptance criteria

1. Walking between buildings follows connected streets.
2. Every building on a terrace borders a street.
