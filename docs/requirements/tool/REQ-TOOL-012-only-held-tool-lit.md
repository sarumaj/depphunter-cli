---
id: REQ-TOOL-012
uuid: e975a287-9c6c-4474-a26d-bdcdd457abc6
title: Only the held tool is lit
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M14
verification:
  - manual
  - inspection
---

## Statement

The walk camera **shall** carry its own lights (key, fill, rim and sky/ground
ambient), and every material in the scene other than the held tool and hand
**shall** be unlit, so that only what the walker holds is lit.

## Rationale

A hand is round and a rod blank has a highlight, while the city keeps its flat,
data-first coloring.

## Acceptance criteria

1. The held tool shows shading and highlights.
2. Buildings, streets and projectiles are drawn with unlit materials.
