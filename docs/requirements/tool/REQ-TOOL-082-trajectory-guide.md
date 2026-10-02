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
**shall** draw the path its shot would take if fired now, and a marker, a ring
with a dot in it, where it would land, lying on the surface it lands on or
facing the eye on a bug. The path **shall** be the shot's own: a stand-in
launched as the shot would be (REQ-TOOL-081), carried by the walker's motion,
and flown down the same steps in the wind blowing now, steering as a tracking
dart steers, so the shot lands at the marker unless what it lands on moves or
the wind turns meanwhile; a nail's scatter is left out. It **shall** be worked
out again every frame. The line **shall** be thin and bright, as wide on screen
near as far, on a soft dark edge that keeps it legible against the sky, and
**shall** run in its own color into the marker. Where a building is in front of
it the path and marker **shall** still show, a little under half as strongly.
Nothing **shall** be drawn where the shot would come down on nothing - the
water, or past its reach - nor for a tool that throws nothing, for the
extinguisher, with the hands put away (H), while the walker is held, the wheel
is open or a line is pulling them. A tool that has the guide **shall** have no
crosshair: what it would catch or tag is what the guide's marker is on.

## Rationale

With shots that drop and drift in the wind, where one lands is something to
judge; a guide turns that from guessing into aiming. A guide worked out beside
the shot drifted from it - the shot was solved onto the crosshair's point and
the guide drawn along the view, the walker's motion and a dart's steering left
out - and a crosshair in the middle of the view said a shot that drops would
land where it does not. Flown by the shot's own steps, the guide is the
path.

## Acceptance criteria

1. A dart lands at its marker standing and running, in still air and in a
   crosswind, and running sideways moves the marker sideways.
2. A tracking dart that steers onto a wall lands at the marker the guide put
   on that wall; a nail into a wall is ringed on its face.
3. Nothing is drawn for a shot out over the water, for the net, the camera and
   the extinguisher, or while the walker is held.
4. A tool with the guide shows no crosshair.
