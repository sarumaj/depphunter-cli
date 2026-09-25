---
id: REQ-HUNT-013
uuid: 6ca3cb94-76ce-4be2-82e3-cd2fbd700c2b
title: Bug shapes modelled in Blender with stand-ins
scope: hunt
type: constraint
priority: must
status: implemented
verification:
  - inspection
  - ui
---

## Statement

The bug shapes **shall** be modelled by `scripts/bug.py` and exported to
`web/static/bug.glb`, and the UI **shall** draw a stand-in for every shape whose
meshes are missing from the loaded file.

## Rationale

A file built before a shape existed must still put that shape on the street, and
nothing waits on the model load.

## Acceptance criteria

1. `scripts/bug.py` exports shell, dark, legs and wing meshes for the beetle, and
   prefixed meshes for the caterpillar and the mite.
2. Without `bug.glb`, all three shapes are still drawn.
