---
id: REQ-CITY-033
title: Balconies, awnings, cornices and rooftop gear
scope: city
type: functional
priority: should
status: implemented
verification:
  - ui
  - manual
---

## Statement

In the city style, buildings **shall** carry details as geometry, placed from
their type (REQ-CITY-031) where the facade paints for them: balcony slabs with
railings under the French windows of residential facades, story by story;
awnings over the shops; cornices under the roof line of brick, art-deco and
mixed buildings and a string course on brick; and on the roofs water tanks,
air-conditioning units, antennas and parapet railings. Details **shall** be
built for the buildings near the walker, or on screen once the map is zoomed in
to at least 40 pixels a unit, and for none of the buildings a selection or a
filter dims. They are decoration: nothing about the walker's collisions,
the grapple or the jetpack changes.

## Rationale

Balconies and awnings are what makes a street read as a street. Built only
where they are more than a pixel, and one draw per kind, they cost little.

## Acceptance criteria

1. Every detail stays within its building's footprint, standing out of it by at
   most 0.075 units (less than the walker's radius); rooftop gear stands within
   the roof and rises at most 0.24 units above it; nothing else rises above
   the building's height.
2. Balconies stand on whole stories, centered on the facade's bays.
3. Walking away from the buildings, or zooming the map out, removes their
   details; a dimmed building has none; a recolored one repaints its details in
   its new color.
4. Outside the city style there are none.
5. Where details are built, the facade stops painting the stand-ins it draws
   otherwise (railings, awning canvas, rooftop plant).

## Notes

`web/static/map/details.js`: `detailsOf` places them, `Details` keeps one
InstancedMesh per kind and rebuilds the instances when the set of buildings
changes, a budget of buildings a frame. Balcony railings are cut out of their
panels up close and drawn as their average once a bar is below a pixel.
