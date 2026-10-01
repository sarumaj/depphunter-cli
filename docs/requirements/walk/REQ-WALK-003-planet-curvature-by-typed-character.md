---
id: REQ-WALK-003
title: Planet curvature keys by typed character
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

In walk mode the character `]` and the `Page Up` key **shall** enlarge the
planet's radius by a factor of 1.25, and the character `[` and the `Page Down`
key **shall** reduce it by the same factor; the brackets are identified by the
character typed (`KeyboardEvent.key`), the page keys by their position. `+` and
`-` **shall** change the map's depth, as they do on the map (REQ-MAP-023).

## Rationale

`+` and `-` are the depth everywhere else, and a walker wants a closer or a
wider look at the city as much as anybody. On a German keyboard the brackets
need AltGr, which walk mode does not take, so the page keys stand in on every
layout.

## Acceptance criteria

1. On a US layout `[` and `]` shrink and grow the planet; `Page Down` and
   `Page Up` do the same on every layout.
2. `+` and `-` change the depth in walk mode, with the walker kept beside the
   building they stand at (REQ-WALK-025).
3. The HUD shows the new radius.

## Notes

The mouse wheel zooms the field of view (REQ-WALK-013) rather than changing the
radius.
