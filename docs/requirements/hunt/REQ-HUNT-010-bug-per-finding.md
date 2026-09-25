---
id: REQ-HUNT-010
uuid: 8809bca4-e689-4c18-9751-baf416790540
title: A bug patrols the building of each finding
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M13
verification:
  - ui
  - e2e
---

## Statement

In walk mode the system **shall** represent each finding as a bug that walks a
lap on the building the finding belongs to, with two exceptions: a
vulnerability the scanner reports as reachable **shall** be drawn as a fire
instead, and at most 140 bugs **shall** walk at once, chosen most severe first.

## Rationale

Findings become something the walker can find and catch in the street. A large
repository reports thousands of findings; drawing each as an animated bug would
cost the frame rate (REQ-PERF-001) and fill the streets, so the worst ones
walk. A reachable vulnerability is more urgent than the rest and is shown as a
fire, which spreads and must be put out.

## Acceptance criteria

1. A finding placed on a file shows a bug walking a lap around, up, on top of or
   above that file's building.
2. The bug keeps walking while it is not caught.
3. With more than 140 unreachable findings, exactly the 140 most severe walk.
4. A finding reported as reachable is drawn as a fire, not as a bug.
5. Every finding, walking or not, is listed in the side panel (REQ-FND-024).

## Notes

Amended during the requirements review: M13 of the design log has a bug for
every finding; the cap (`MAX_BUGS` in `bugs.js`) and the fires (`fires.js`)
were added later and are kept. Laps lie on the street, a wall band, the roof or
in the air.
