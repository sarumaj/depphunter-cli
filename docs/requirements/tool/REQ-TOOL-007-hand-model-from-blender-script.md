---
id: REQ-TOOL-007
title: Hand model prepared by a committed script
scope: tool
type: constraint
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The walker's hands and forearms **shall** be a rigged model prepared by
`scripts/hand.py` in Blender and exported by `scripts/body.py` to
`web/static/body.glb` - whole for the first-person view, and as the hands of the
body (REQ-WALK-059) - and the scripts **shall** be committed as the model's
source.

## Rationale

A model that can be read, reviewed and regenerated is maintainable; a binary
alone is not.

## Acceptance criteria

1. `scripts/body.py` runs under Blender or `bpy` and writes `web/static/body.glb`,
   with the hand from `scripts/hand.py`.
2. The UI draws no hand geometry of its own.
