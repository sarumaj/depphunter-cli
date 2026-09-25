---
id: REQ-HUNT-043
uuid: 705f712f-6277-41fa-93ea-d142a2e50dfd
title: Raised photograph fills the view
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

At rest in the raised pose, the camera's screen **shall** be square on to the
eye, centred on the line of sight, cover about nine tenths of the view's height,
and have no part of the tool nearer the eye than the screen; the arm **shall**
run past the eye and be clipped there.

## Rationale

The pose is solved rather than chosen: at a slant the picture is read at a
slant, and anything nearer than the screen is cut by the near plane.

## Acceptance criteria

1. The screen's centre is within 0.004 of the line of sight.
2. The screen's normal is within 3 degrees of the view direction.
3. The screen covers more than 80 % and at most 100 % of the view's height.
4. No part of the tool is nearer than 1.4 times the near plane distance.
