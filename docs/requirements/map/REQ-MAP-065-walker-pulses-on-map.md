---
id: REQ-MAP-065
title: The walker's place on the map pulses
scope: map
type: functional
priority: may
status: implemented
verification:
  - unit
  - manual
---

## Statement

The figure that shows on the map where the walker stands **shall** pulse: a
ring in the figure's color **shall** spread out over the ground from its feet
and fade, over and over, about every second and a half, and be seen through
whatever stands in front of it. The map **shall** keep redrawing for the pulse
only while the figure is shown. Where the system asks for reduced motion the
ring **shall** stand still around the figure.

## Rationale

A small figure in a street between towers is easy to lose on a large map, and
coming back from the street the first question is where you were. Something
that moves is found at a glance where a still mark has to be searched for.

## Acceptance criteria

1. Back on the map, a ring spreads from the walker's figure and fades, again
   and again, and shows through a tower in front of it.
2. Walking in again, the map stops being redrawn for the pulse.
