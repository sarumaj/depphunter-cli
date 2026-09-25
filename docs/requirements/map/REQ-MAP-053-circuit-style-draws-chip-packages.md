---
id: REQ-MAP-053
uuid: 3b463007-6ac8-4038-83d3-de23d4768b17
title: Circuit style draws chip packages
scope: map
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

In the circuit style, the system **shall** draw buildings as chip packages: an
epoxy body with a parting line, a row of pins along the foot of every face, a
printed part number, and a finned heatsink instead when the part is taller than
2.2 units.

## Rationale

The board's parts stand where the city's buildings stand.

## Acceptance criteria

1. A building in the circuit style shows pins and a part number; a tall one
   shows a heatsink.
