---
id: REQ-TOOL-009
title: Hand rig posed by bone name
scope: tool
type: interface
priority: must
status: implemented
verification:
  - inspection
  - manual
---

## Statement

The hand model **shall** be rigged with a bone per phalanx, and the UI **shall**
pose it by bone name only; the bone names **shall** be the contract between
`scripts/hand.py` and `web/static/walk/hands.js` - and `web/static/walk/body.js`,
whose hands `scripts/body.py` rigs with the same names and a side's suffix.

## Rationale

Posing by name decouples the UI from the model's internal structure.

## Acceptance criteria

1. Every finger has proximal, intermediate and distal phalanx bones named as
   `hands.js` expects.
2. `hands.js` refers to bones only by name.

## Notes

The names are the WebXR joint names (for example
`index-finger-phalanx-proximal`).
