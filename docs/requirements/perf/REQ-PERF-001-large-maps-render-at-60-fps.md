---
id: REQ-PERF-001
uuid: 42dcc373-fb7b-457a-baaf-79a5d3991044
title: Large maps render at 60 fps
scope: perf
type: non-functional
priority: must
status: implemented
verification:
  - e2e
  - manual
---

## Statement

The system **shall** render and navigate a map of about 10,000 files at 60
frames per second, drawing all boxes of a layout as one instanced mesh.

## Rationale

One draw call per box does not scale to a large repository.

## Acceptance criteria

1. Panning and zooming the fully expanded map of a 10,000-file repository keeps
   60 fps on a current desktop GPU.
2. The boxes of a layout are drawn by a single instanced mesh.
