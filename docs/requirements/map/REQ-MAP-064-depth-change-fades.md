---
id: REQ-MAP-064
title: A change of depth blinks
scope: map
type: functional
priority: should
status: implemented
verification:
  - ui
  - manual
---

## Statement

When the depth changes (REQ-MAP-023), on the map and in walk mode alike, the
view **shall** blink: two black lids **shall** close over the city and its
labels from the top and the bottom in about 0.14 s, the city **shall** be laid
out again while they are shut, and they **shall** open on it over about 0.22 s.
The lids are black in every theme. With `prefers-reduced-motion: reduce` there
**shall** be no blink.

## Rationale

A change of depth moves every building at once; seen as a jump, nothing in the
new city can be followed from the old one, and a short blink reads as one view
becoming the other. It was a fade of the city's opacity, which showed the page
behind it - white in a light theme - so the city flashed white in between: a
glitch rather than a transition. Closing the eyes on one depth and opening them
on the next is what a walker does, and black is what is seen between.

## Acceptance criteria

1. Pressing `+` closes the lids over the city, and they open on the new layout.
2. Nothing white or light is seen in between, in either theme.
3. The walk HUD stays visible throughout; only the city and its labels are
   covered.
4. With reduced motion the new layout appears without a blink.
