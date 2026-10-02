---
id: REQ-TOOL-082
title: The path a shot would take is shown before it is taken
scope: tool
type: functional
priority: should
status: implemented
verification:
  - unit
  - manual
---

## Statement

While a tool that throws or fires something is in the right hand, walk mode
**shall** draw the path its shot would take if fired now - a thin bright line
from the muzzle, as wide on screen near as far, on a soft dark edge that keeps
it legible against the sky - and a marker, a ring with a dot in it, where it
would land, lying on the surface it lands on. The path **shall** be the one the
shot would fly (REQ-TOOL-081): to the mark along the solved path when the
crosshair is on something the tool can reach and the air allows it; otherwise
along the view under the shot's physics in the wind blowing now, to the first
thing it strikes, the water, or the end of the tool's reach, with no ring past
its reach. It **shall not** be drawn for a tool that throws nothing, for the
extinguisher, with the hands put away (H), while the walker is held, the wheel
is open or a line is pulling them. Where a building is in front of it the path
and marker **shall** still show, faintly, so the whole path can always be read.

## Rationale

With shots that drop and drift in the wind, where one lands is something to
judge; a guide turns that from guessing into aiming. A tracking dart that
misses steers onto a wall on the way, which the guide cannot know in advance:
it shows the unsteered path.

## Acceptance criteria

1. A dart lobbed at nothing lands where its ring was, in still air and in a
   crosswind, and the ring moves with the wind.
2. An aimed shot's ring is on the aimed point; a shot into a wall is ringed on
   the wall's face.
3. The net, the camera and the extinguisher show no path.
