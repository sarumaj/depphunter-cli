---
id: REQ-CITY-001
uuid: a493581e-29b9-4d7a-8443-312908243d28
title: Procedural city look modulating data colors
scope: city
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M7
verification:
  - manual
  - inspection
---

## Statement

The city look of the boxes **shall** be produced by procedural shaders without
image assets, and facades and roofs **shall** be computed from each box's data
color (language, size, history, dimming, hover), modulating it and never
replacing it.

## Rationale

The map's colors carry data; the city must not hide them.

## Acceptance criteria

1. Buildings of two languages keep their two language colors under the facade
   texture.
2. Hovering a building still changes its color visibly in both views.

## Notes

M7 kept the isometric view unchanged; since M10 the city look is shared by both
views (REQ-CITY-002).
