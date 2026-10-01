---
id: REQ-MAP-057
title: Galaxy style props and sky
scope: map
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

In the galaxy style, the system **shall** place, where the city has trees,
seven kinds of thing grown out of the dark: three of crystal (a spire, a
cluster, a squat point), a fungus and a seed pod that light themselves, a
ringed stone and a slab of obsidian; where it has bushes, rubble, sprouts of
crystal and cinders; and where it has lamps, beacons. The fungi, the pods and
the beacons **shall** glow by day and by night, more strongly at night, and a
glow **shall** keep its color after dark. The style **shall** draw nebulae
and a star field instead of the sea and the sky.

## Rationale

The environment belongs to the style. Three crystals, one rubble and an unlit
beacon repeated across a map read as a pattern; a few lights among them read
as a place, the way the city's lamps do at night.

## Acceptance criteria

1. A galaxy map shows beacons on its terrace edges and stars in its water; walk
   mode shows a star field and nebulae instead of a sky.
2. Every kind of prop is drawn somewhere on a map with enough of them; the
   fungi, pods and beacons are haloed by day and at night.
