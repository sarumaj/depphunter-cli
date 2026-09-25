---
id: REQ-MAP-019
uuid: 7493f4b1-75ec-463b-9888-3590979593e9
title: Fit and reset view actions
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
verification:
  - manual
  - e2e
---

## Statement

The UI **shall** offer a fit action (toolbar button and `Home`) that centers the
whole map and zooms so that it fills the view, and a reset action that returns
the camera to the initial isometric orientation and fit.

## Rationale

A way back to the whole map is needed after any amount of panning, zooming and
orbiting.

## Acceptance criteria

1. `Home` or the Fit button shows the whole map centered.
2. A reset action returns the camera to the orientation and zoom it had on load.

## Notes

Fixed: `MapScene.reset` returns to the first isometric quarter and its elevation
(`setIso(0)`) and fits the map, as on load. It is the toolbar's Reset button and
the `R` key in the map view (walk mode's `R`, the tool wheel, is not affected),
and the help lists it.
