---
id: REQ-WALK-053
title: What is being looked at stays in focus
scope: walk
type: functional
priority: should
status: implemented
verification:
  - unit
  - manual
---

## Statement

While the walker is held to read a building's details, or a finding caught on
it, the blur over the street **shall** leave that building sharp, together with
a margin of about 36 CSS pixels around its outline on screen, the blur coming
back in over a soft edge. While a bug's catch plays out, the street **shall**
be blurred except for a soft-edged circle that follows the bug and is sized by
how near it is. Everything else that holds the walker - the backpack, a menu,
the tool wheel - **shall** keep the whole view blurred.

## Rationale

The blur says the city is waiting; what is being read about, or the catch that
just happened, is the one part of it still worth looking at, the way a lens
focused on it would show it.

## Acceptance criteria

1. With a building's details open, the building and the street just around it
   are sharp and the rest of the view is blurred.
2. A building with too little of it on screen leaves the blur whole.
3. During a catch, a circle around the bug is sharp and follows it; the blur
   goes when the catch is over.
4. With the backpack open, the whole view is blurred.

## Notes

The building's outline is the convex hull of its projected corners, cut out of
an SVG mask on the veil; the bug's circle is a CSS radial-gradient mask placed
by custom properties.
