---
id: REQ-TOOL-012
title: Only the held tool is lit
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
  - inspection
---

## Statement

The walk camera **shall** carry its own lights (key, fill, rim and sky/ground
ambient), and every material in the scene other than the held tool and hand
**shall** be unlit, so that only what the walker holds is lit. The lights'
directions **shall** be fixed relative to the camera, so what is held is lit
the same wherever the walker stands and whichever way it faces.

## Rationale

A hand is round and a rod blank has a highlight, while the city keeps its flat,
data-first coloring.

## Acceptance criteria

1. The held tool shows shading and highlights.
2. Buildings, streets and projectiles are drawn with unlit materials.
3. The key light comes from the left, above and behind the lens, and every
   light's direction in view space and the shading of a surface facing any
   way in front of the lens are the same near the middle of the map and
   hundreds of units away from it, facing other ways.
