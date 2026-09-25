---
id: REQ-WALK-003
uuid: 517021ba-a3a6-4a12-a1aa-ae8d147a0f70
title: Planet curvature keys by typed character
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M7
  - docs/REQUIREMENTS.md M10
verification:
  - manual
---

## Statement

In walk mode the characters `]`, `+` and `=` **shall** enlarge the planet's
radius by a factor of 1.25 and the characters `[`, `-` and `_` **shall** reduce
it by the same factor, identified by the character typed (`KeyboardEvent.key`)
rather than by the key's position on the keyboard.

## Rationale

On a German keyboard the key at `BracketRight` types `+`; binding by key
position made `+` curve the planet while `-` still changed the map's depth.

## Acceptance criteria

1. On a US layout `[` and `]` shrink and grow the planet.
2. On a German layout `+` and `-` both act on the planet, not on the map depth.
3. The HUD shows the new radius.

## Notes

M7 also let the mouse wheel change the radius; since M10 the wheel zooms the
field of view (REQ-WALK-013).
