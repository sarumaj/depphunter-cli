---
id: REQ-CITY-031
title: Building types
scope: city
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

In the city style, every building, district, package and symbol plot **shall**
be drawn as one of seven types - a residential slab with balconies, an office
tower with a glass curtain wall, a brick walk-up, a concrete panel block, an
art-deco tower with setbacks, a warehouse, and a mixed-use building with shops
under flats - chosen from a hash of its node's id and from its proportions only.
Facades **shall** be drawn in the walker's proportions: a story 0.84 units high
(2.8 times the 0.3 a facade is laid out with), a door a little taller than the
walker, with the windows, balconies, awnings and cornices scaled alike.

## Rationale

A city of one building repeated reads as a texture, not a city. The type is
form and material, never color, so variety cannot be mistaken for data; and it
depends on nothing that changes between runs or live updates, so a file is the
same building every time it is seen.

## Acceptance criteria

1. The same box gets the same type on every run, whatever its position, color
   or dimming.
2. Every type occurs on a large map.
3. Only boxes lower than 1.3 units and at least 0.9 units across are
   warehouses; only boxes at least 5.4 units high are art-deco towers; symbol
   plots are brick, panel or office pavilions.

## Notes

`web/static/map/buildings.js` `archetype` chooses the type; the shaders read it,
with a per-building variant, from the `aBuild` attribute. Kinds of file (a
binary, a configuration file, documentation) do not have types of their own:
the box kinds the map already has decide the pavilions (symbols), and the
rest follows the proportions.
