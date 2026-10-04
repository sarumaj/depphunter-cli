---
id: REQ-HUNT-021
title: Tracker centered on and turning with the walker
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
  - manual
---

## Statement

Walk mode **shall** show a tracker: a top-down circular sweep centered on the
walker, rotated so that the walker's facing direction points up, marking the
walker's field of view. Under the sweep it **shall** show the map, faded well back
behind everything marked on it: the land against the water, and the buildings
standing on it.

## Rationale

Without it a bug cannot be found on a map of a thousand files.

## Acceptance criteria

1. Turning the walker rotates the tracker's contents; the walker marker always
   points up.
2. The tracker is hidden when there is nothing on the map to track.
3. The islands and the buildings near the walker show faintly on the tracker,
   turning with it, so the shore and the streets can be steered by.
