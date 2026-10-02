---
id: REQ-CITY-030
title: Sky and planet water in walk mode
scope: city
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

Walk mode **shall** draw a sky dome that follows the camera and a water sphere
at the land boxes' base as the planet's surface, with slow procedural ripples.

In the city by day the sky **shall** be deepest blue overhead, paler and hazier
towards the horizon and warmer round the sun, with two layers of cloud: cumulus
with defined, broken edges, lit on the side towards the sun and gray-blue in
their own shadow, rimmed with light where the sun is behind them; and thin
cirrus streaks above them. Both **shall** drift with the walk mode wind
(REQ-TOOL-081). The sun's disc **shall** be dimmed behind a cloud, its glow
showing through. By night the same clouds are dark against the stars.

## Rationale

Sky and water turn a curved map into a small planet. The daylight sky was a
two-color gradient under flat, blurred wisps, which read as a texture rather
than as weather.

## Acceptance criteria

1. Walk mode shows a sky above the horizon and rippling water around the
   islands.
2. Looking away from the sun, the clouds are bright on top and darker
   beneath; looking towards it, their edges are lit.
3. The clouds move the way the HUD's wind arrow points.
