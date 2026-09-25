---
id: REQ-HUNT-041
uuid: fa0f1681-4d12-402e-b590-a2e524929a08
title: Screenshots keep the whole screen
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M28
verification:
  - e2e
---

## Statement

A screenshot **shall** contain the view as shown, including the held tool, the
selection, the outline and the arcs.

## Rationale

A screenshot is the screen, not what the camera was pointed at.

## Acceptance criteria

1. A screenshot still shows the hand, the selection and the arcs (M28
   acceptance).
