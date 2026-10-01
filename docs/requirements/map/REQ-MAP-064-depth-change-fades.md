---
id: REQ-MAP-064
title: A change of depth fades
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
city and its labels **shall** fade out in about 0.14 s, be laid out again while
they cannot be seen, and fade back in over about 0.22 s. With
`prefers-reduced-motion: reduce` there **shall** be no fade.

## Rationale

A change of depth moves every building at once; seen as a jump, nothing in the
new city can be followed from the old one, and a short fade reads as one view
becoming the other.

## Acceptance criteria

1. Pressing `+` fades the city out, and the new layout fades in.
2. The walk HUD stays visible throughout; only the city and its labels fade.
3. With reduced motion the new layout appears without a fade.
