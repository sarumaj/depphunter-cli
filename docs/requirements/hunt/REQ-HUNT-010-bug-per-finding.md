---
id: REQ-HUNT-010
uuid: 8809bca4-e689-4c18-9751-baf416790540
title: A bug patrols the building of each finding
scope: hunt
type: functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md M13
verification:
  - ui
  - e2e
---

## Statement

In walk mode the system **shall** represent every finding as a bug that walks a
lap on the building the finding belongs to.

## Rationale

Findings become something the walker can find and catch in the street.

## Acceptance criteria

1. A finding placed on a file shows a bug walking a lap around, up, on top of or
   above that file's building.
2. The bug keeps walking while it is not caught.

## Notes

Partial: at most the 140 most severe findings walk (MAX_BUGS in bugs.js), and a
vulnerability that the scanner reports as reachable is drawn as a fire
(fires.js) instead of a bug. Laps lie on the street, a wall band, the roof or in
the air; neither the cap, the fires nor the lap kinds are in the design log.
