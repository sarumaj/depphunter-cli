---
id: REQ-UI-004
uuid: 1c1cfde4-201f-43c2-9db4-32066bb943af
title: First-visit introduction of five cards
scope: ui
type: functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md M22
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

Partial: the second card still says that selecting draws "roads through the
streets ... with chevrons", which M24 removed; selecting draws arcs only.
