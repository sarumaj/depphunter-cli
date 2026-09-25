---
id: REQ-TOOL-059
uuid: 6f8cd986-587b-42e3-a25d-4dcaa0d7fcc7
title: Wheel flick: hold, throw, release
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M31
verification:
  - ui
  - e2e
---

## Statement

Releasing `R` while the wheel's cursor is on a wedge **shall** take that wedge's
tool and close the wheel.

## Rationale

Flicked, the wheel is one gesture.

## Acceptance criteria

1. `R` held with a flick to the left and a release puts a grapple gun in the off
   hand (M31 acceptance).
