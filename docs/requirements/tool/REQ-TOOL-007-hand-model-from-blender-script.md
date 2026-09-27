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
`scripts/hand.py` in Blender and exported to `web/static/hand.glb`, and the script
**shall** be committed as the model's source.

## Rationale

A model that can be read, reviewed and regenerated is maintainable; a binary
alone is not.

## Acceptance criteria

1. `scripts/hand.py` runs under Blender or `bpy` and writes `web/static/hand.glb`.
2. The UI draws no hand geometry of its own.
