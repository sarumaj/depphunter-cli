---
id: REQ-CITY-034
title: Setback towers keep the height of their file
scope: city
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

In the city style, an art-deco tower **shall** be drawn as a shaft on its whole
footprint up to 0.72 of its height and two tiers standing on it, each inset by
0.12 of the footprint on every side, the top of the highest at exactly the
building's height. The walker **shall** stand on the tiers and ledges where
they are, and the tiers **shall** take the building's color, dimming and
picking.

## Rationale

Setbacks are the silhouette of a skyline, but height is data: the top stays
where the file's line count puts it, and the footprint stays the layout's.

## Acceptance criteria

1. The highest tier's top equals the building's height, and each tier stands
   on the one under it, inside it.
2. In the city the walker's ground over the shaft's ledge is the shaft's top;
   in the other styles the box is whole.
3. Clicking a tier selects its building.
