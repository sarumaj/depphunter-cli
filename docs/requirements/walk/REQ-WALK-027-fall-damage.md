---
id: REQ-WALK-027
uuid: 659f0c46-5985-43e4-b977-0b0137f0676b
title: Fall damage
scope: walk
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

A fall of more than 3 map units **shall** cost health in proportion to the part
of the drop beyond 3 units (26 points per unit), measured from the top of the
fall; a shorter fall **shall** cost nothing.

## Rationale

Roofs are within reach of the grapple and the jet; arriving on one is only worth
something if leaving it the wrong way costs something. A terrace wall costs
nothing.

## Acceptance criteria

1. A drop of 2 units costs nothing.
2. A drop of 6 units costs something and is survived at full health.
3. A drop of 40 units kills.
4. A walker who steps off a tower dies and is returned to the map.
