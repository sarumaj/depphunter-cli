---
id: REQ-HUNT-004
uuid: 5b8768d2-45ed-4da7-88b8-bc9bc4057387
title: Beacon colored by the worst finding
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M21
verification:
  - e2e
  - manual
---

## Statement

A tagged module's beacon **shall** take the severity color of the worst finding
in the module; where the module has no findings, or findings are turned off, the
beacon **shall** keep its own default color.

## Rationale

What the hunt has collected then says which modules were worth having. Walk mode
works the same with findings on or off.

## Acceptance criteria

1. A tagged module containing a critical finding shows a beacon in the critical
   severity color (M21 acceptance).
2. With findings turned off, every beacon is the default beacon color.
