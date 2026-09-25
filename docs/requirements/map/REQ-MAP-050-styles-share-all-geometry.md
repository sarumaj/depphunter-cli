---
id: REQ-MAP-050
uuid: 26fddb39-652c-4eea-914e-278cce9b80c4
title: Styles share all geometry
scope: map
type: constraint
priority: must
status: implemented
verification:
  - manual
  - inspection
---

## Statement

All styles **shall** share the same layout, street network, ramps and bridges,
and **shall** differ only in the surface painter selected by one shader uniform
(`uStyle`) and in the props placed on the map; switching style **shall** cost
one prop rebuild and no relayout.

## Rationale

A style is a look, not a second renderer: one geometry keeps the views
comparable and the cost of switching small.

## Acceptance criteria

1. The same repository in all three styles keeps every box in the same place.
2. Switching style does not rebuild the layout.
