---
id: REQ-CITY-006
uuid: a122a785-e222-47f2-b491-2bb14c4f8065
title: Streets as the free space of terrace tops
scope: city
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M8
verification:
  - manual
---

## Statement

The top of every terrace **shall** be drawn as streets wherever it is not
covered by a child box: gaps between children **shall** be side streets and the
padding along the terrace edge a ring road, so streets connect by construction.

## Rationale

Deriving streets from free space needs no routing and cannot leave a building
unconnected.

## Acceptance criteria

1. Walking between buildings follows connected streets.
2. Every building on a terrace borders a street.
