---
id: REQ-CITY-032
title: Facade detail and the data color
scope: city
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

City facades **shall** show window frames and mullions in proportions of the
building's type; blinds or curtains in some windows; rooms that darken towards
their middle; glass that reflects the sky more the flatter it is seen;
shopfronts with awnings and entrance doors on the ground floor; weathering
stains, and shading at the foot, in the corners and under cornices; and, at
night, lit windows warm or cool by floor. What a facade averages to from afar
**shall** keep the hue of the box's color.

## Rationale

A building is recognized as one by its windows and its street floor, and the
map is read by its colors: the detail must never take the color away.

## Acceptance criteria

1. By day, a facade's far average differs in hue from the box's color by less
   than 12 degrees for every color the palettes give a building, and its
   brightness stays between half and 1.3 times the color's (1.45 times for the
   darkest colors).
2. At night (the dark theme's map), the hue differs by less than 30 degrees.
3. Per-building variation changes brightness and roughness only.
4. Detail fades to the far average as its pixel footprint shrinks, without
   aliasing or moire.

## Notes

The far average is `facadeFar` in both `web/static/buildings.js` and the city
shader, built from one `LOOKS` table. A lit window counts for half its
brightness from afar, so the dark theme's map keeps its legend.
