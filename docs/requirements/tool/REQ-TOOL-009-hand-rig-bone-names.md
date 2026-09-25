---
id: REQ-TOOL-009
uuid: 8c52f2c9-6616-4823-914f-cf77c1147de5
title: Hand rig posed by bone name
scope: tool
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M14
verification:
  - inspection
  - manual
---

## Statement

The hand model **shall** be rigged with a bone per phalanx, and the UI **shall**
pose it by bone name only; the bone names **shall** be the contract between
`tools/hand.py` and `web/static/hands.js`.

## Rationale

Posing by name decouples the UI from the model's internal structure.

## Acceptance criteria

1. Every finger has proximal, intermediate and distal phalanx bones named as
   `hands.js` expects.
2. `hands.js` refers to bones only by name.

## Notes

The names are the WebXR joint names (for example
`index-finger-phalanx-proximal`).
