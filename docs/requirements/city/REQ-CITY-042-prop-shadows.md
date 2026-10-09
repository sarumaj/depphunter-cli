---
id: REQ-CITY-042
title: What stands on the ground shades it
scope: city
type: functional
priority: may
status: implemented
verification:
  - unit
  - manual
---

## Statement

Every tree, bush, street lamp and park amenity - a playground's swings,
seesaws, spring riders, roundabouts, slides and bobby cars, a court's goals,
hoops and net - **shall** have a shadow on the ground under it, in every style
and in both views: darkest under its middle and fading to nothing at its rim, a
tree's as wide as its crown, an amenity's over the ground it stands on and
turned with it, lighter than a tree's since most of one is air.

A shadow **shall** darken what it lies on - the lawn, the pavement, the paint of
a court - rather than cover it, **shall** fade out into the fog rather than
take on its color, **shall** stay a shadow at night rather than go black, and
**shall** be drawn, culled and coarsened with the prop it belongs to
(REQ-PERF-010).

## Rationale

The map is unlit: nothing in it casts a shadow, so a tree stood on a lawn as if
pasted on it. A soft dark patch where something meets the ground is what puts
it there, and costs one more instanced draw a kind of prop.

## Acceptance criteria

1. Walking through a park, every tree, bush, lamp post and playground amenity
   has a soft dark patch on the ground under it.
2. A patch darkens the court's lines and the lawn under it alike.
3. At night the patches are still soft shadows, not black discs.
4. Far down a street in fog, the patches fade out with the ground.
