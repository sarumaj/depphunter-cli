---
id: REQ-PERF-012
title: Shaders compiled ahead of the frames that need them
scope: perf
type: non-functional
priority: should
status: implemented
verification:
  - ui
  - manual
---

## Statement

Once a layout is drawn, the system **shall** compile, while the page is idle,
the shader programs walk mode needs and those the boxes need in the other map
styles, one at a time and in parallel with drawing where the browser allows,
and **shall** leave the map's style and fog as they were.

## Rationale

A program is otherwise compiled on the first frame that needs it, and a shader
the size of the city's holds that frame up for a second or more on some
drivers: the first step into walk mode or the first frame of another style
would stall.

## Acceptance criteria

1. After a layout, walk mode's fog is compiled for every material, then the
   boxes for each other style.
2. The style and the fog are as they were afterwards, and every material
   finds its current program again on the next frame.
3. Nothing is compiled ahead while walking, and a later layout's warm-up
   replaces an earlier one still running.
