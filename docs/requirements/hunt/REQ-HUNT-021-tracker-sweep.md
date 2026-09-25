---
id: REQ-HUNT-021
uuid: 5875a0df-3991-444b-8020-25566e35a62c
title: Tracker centred on and turning with the walker
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M14
verification:
  - e2e
  - manual
---

## Statement

Walk mode **shall** show a tracker: a top-down circular sweep centred on the
walker, rotated so that the walker's facing direction points up, marking the
walker's field of view.

## Rationale

Without it a bug cannot be found on a map of a thousand files.

## Acceptance criteria

1. Turning the walker rotates the tracker's contents; the walker marker always
   points up.
2. The tracker is hidden when there is nothing on the map to track.
