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
covered by a child box: gaps between children **shall** be side streets and the
padding along the terrace edge a ring road, so streets connect by construction.
A terrace stacked higher is a plaza instead (REQ-CITY-037).

## Rationale

Deriving streets from free space needs no routing and cannot leave a building
unconnected.

## Acceptance criteria

1. Walking between buildings follows connected streets.
2. Every building on a terrace borders a street.
