---
id: REQ-CITY-005
title: Dimmed boxes drawn faded
scope: city
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

Boxes dimmed by a selection or by the legend **shall** be marked by a per-box
fade flag next to their color and **shall** be drawn blended 72 % towards their
plain color, so the focus stands out.

## Rationale

A dimmed city must still read as dimmed, yet keep its texture.

## Acceptance criteria

1. Selecting a building leaves unrelated buildings visibly fainter than the
   neighborhood.

## Notes

Facades and roofs stay on dimmed boxes, only much fainter (`FADE` 0.72 in
`city.js`).
