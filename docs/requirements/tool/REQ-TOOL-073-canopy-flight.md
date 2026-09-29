---
id: REQ-TOOL-073
title: A canopy flies on quadratic drag and lift
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

Under an open canopy the walker **shall** move on gravity and on the canopy's
drag and lift, both in proportion to the square of the airspeed and set by how
open the canopy is, settling to a descent of 2.5 to 3.2 meters a second and a
glide of 3.6 to 4.4 at the toggles' neutral trim. `W` **shall** trim it faster and
steeper, and `S` slower and sinking a little more; `A` and `D` (or the arrow
keys) **shall** turn it, banking into the turn and sinking faster for it, and
the walker **shall** turn with it. The flight **shall** be the same whatever the
frame rate.

## Rationale

A canopy is flown, not pointed: it has its own speed and heading, and what the
keys do is trim it and steer it. A descent that depended on the frame rate would
land somewhere else on every screen.

## Acceptance criteria

1. A canopy in steady flight sinks at 2.5 to 3.2 meters a second and glides
   3.6 to 4.4 to 1.
2. `W` is faster and steeper, `S` slower and sinking more, a turn sinks at least
   a fifth faster and banks.
3. An eight-second flight at 60 frames a second, at 144 and on an uneven clock
   stays within two substeps of travel of itself.

## Notes

Integrated in fixed substeps of 1/240 of a second (`web/static/parachute.js`).
