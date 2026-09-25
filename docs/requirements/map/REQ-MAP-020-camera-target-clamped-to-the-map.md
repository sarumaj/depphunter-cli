---
id: REQ-MAP-020
uuid: 3d8d85d6-10e5-4f55-b8b6-415775ce665b
title: Camera target clamped to the map
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M8
verification:
  - manual
  - e2e
---

## Statement

The system **shall** keep the isometric camera's target within the map's bounds
extended on every side by a quarter of the map's larger horizontal extent plus 6
units.

## Rationale

Panning must not be able to leave the map behind in empty water.

## Acceptance criteria

1. Panning as far as possible in any direction leaves part of the map on screen.
2. The limits follow the map after a relayout.

## Notes

The design log states the margin as "a quarter of its size (at least 6 units)";
the code adds the two (`PAN_MARGIN_MIN + PAN_MARGIN * size`). The walker's
bounds are REQ-WALK requirements.
