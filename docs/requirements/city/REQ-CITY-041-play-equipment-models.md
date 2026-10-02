---
id: REQ-CITY-041
title: Play equipment modeled where shapes matter
scope: city
type: functional
priority: may
status: implemented
verification:
  - manual
---

## Statement

The parts of the parks' play equipment that posts, bars and blocks do not do
justice to **shall** be models prepared by `scripts/play.py` in Blender and
exported to `web/static/play.glb`, the script committed as their source: a
swing's seat as a rubber belt sagging between its chains, a spring rider as a
horse with a saddle and a handlebar, a basketball net hanging from each rim, and
a goal's net as strings over its top, back and sides. Each **shall** be painted
in the map's flat colors, as the style's kit paints everything else, and until
the file is in - or without it - the amenities **shall** be drawn with the
stand-ins they had.

A slide's chute **shall** leave its platform level, be steepest halfway and
run out level, and a walker going down it **shall** follow that curve.

## Rationale

A goal's net drawn as a grey sheet and a rider drawn as two spheres read as
placeholders up close, which is where the walker now is - sitting on them and
playing at them. Frames and posts are bars in life too, and stay as they are.

## Acceptance criteria

1. Up close, a goal shows a net of strings, a hoop a net, a swing a belt seat
   and a spring rider a horse; with play.glb missing they are as before.
2. A slide's chute curves, and riding it follows the curve.
