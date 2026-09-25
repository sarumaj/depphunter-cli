---
id: REQ-WALK-037
uuid: cd20a84a-b6d4-4e65-93a1-cac9e85d5902
title: No pointer grab under a modal
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

The walker **shall not** request pointer lock while a dialog, the backpack, the
photographs, the export menu or the details panel wants the pointer.

## Rationale

A reticle that grabs the pointer under a dialog leaves a dialog nobody can click
and a street that answers every click instead.

## Acceptance criteria

1. The walk introduction can be clicked through without the mouse being taken.
2. The export menu opens the first time it is asked for from the street.
