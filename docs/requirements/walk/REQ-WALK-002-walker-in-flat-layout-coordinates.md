---
id: REQ-WALK-002
uuid: 9c412998-d4c6-48e9-b24a-13536b909bcd
title: Walker in flat layout coordinates, bent by the renderer
scope: walk
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M7
verification:
  - inspection
  - manual
---

## Statement

The walker's position, collisions, heights and aiming **shall** be computed in
the flat layout coordinates of the isometric map, and only the renderer
**shall** bend the drawn world onto a small planet centred under the walker.

## Rationale

Decision (M7): one layout serves picking, collisions and the isometric view;
bending is a vertex-shader concern (`bendWorld`), and aiming unbends the view
ray back onto the flat map.

## Acceptance criteria

1. Walk mode draws the layout curved away towards a horizon around the walker.
2. The walker's collision and height queries take flat-map coordinates only.
3. Leaving walk mode shows the unchanged isometric layout.
