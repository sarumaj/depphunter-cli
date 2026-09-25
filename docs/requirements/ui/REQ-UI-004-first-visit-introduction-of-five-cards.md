---
id: REQ-UI-004
uuid: 1c1cfde4-201f-43c2-9db4-32066bb943af
title: First-visit introduction of five cards
scope: ui
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

On a first visit, the UI **shall** open an introduction of five cards, stepped
with Back and Next: what the shapes stand for, what selecting does, that the map
can be walked into, what the bugs are, and where the help is.

## Rationale

A map of a repository is not a thing anyone has seen before, so the shapes have
to be said out loud once.

## Acceptance criteria

1. A first load opens the introduction on its first card.
2. Back is disabled on the first card and the last card offers Start, which
   closes it.
3. Each card describes the current behavior of the map.

## Notes

Fixed: the second card now says that selecting draws an arc to each dependency
with an arrow for its direction, fades everything else, and opens the side panel
listing both directions. It had described dependency roads and chevrons, which
are gone. The tour test checks that the card mentions neither.
