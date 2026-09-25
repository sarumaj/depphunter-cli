---
id: REQ-MAP-019
uuid: 7493f4b1-75ec-463b-9888-3590979593e9
title: Fit and reset view actions
scope: map
type: functional
priority: must
status: partial
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

Partial: fit is implemented (`MapScene.fit`); there is no separate reset action.
Rotating with `Q`/`E` restores the isometric elevation after a free orbit, and
fit restores the zoom, but the initial azimuth is not restored by any single
action.
