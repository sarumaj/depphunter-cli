---
id: REQ-WALK-014
uuid: 00356273-c6bc-462d-b2af-e6f38c20566c
title: Left button uses the primary tool
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - manual
---

## Statement

With the pointer locked, pressing the left mouse button **shall** use the
walker's primary tool at the reticle.

## Rationale

The mouse looks, the left button fires: first-person-shooter habit.

## Acceptance criteria

1. A left click with the pointer locked uses the tool once.
2. A left click without the lock asks for the lock instead (REQ-WALK-010).

## Notes

What the tool does is scope `tool`/`hunt`.
