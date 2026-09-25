---
id: REQ-CITY-005
uuid: a3faa85f-8c27-4c9c-b978-acc88f4a4978
title: Dimmed boxes drawn faded
scope: city
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
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
   neighbourhood.

## Notes

The design log says dimmed boxes are drawn plain, without windows. The code
keeps the facades and roofs, only much fainter (`FADE` 0.72 in `city.js`).
