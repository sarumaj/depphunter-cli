---
id: REQ-HUNT-023
uuid: efca4ae6-f97a-4abd-8218-8416104b7d09
title: Tagged modules as rings on the tracker
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
  - manual
---

## Statement

The tracker **shall** draw every tagged module within range as a ring in the
same color as its beacon.

## Rationale

The tracker and the beacons tell one story about where the hunt has been.

## Acceptance criteria

1. A tagged module near the walker appears as a ring.
2. The ring's color equals the module's beacon color (REQ-HUNT-004).
