---
id: REQ-MAP-042
uuid: ce4f45f4-df29-4cb6-8512-dce1ed3b928c
title: Labels placed greedily without overlaps
scope: map
type: functional
priority: must
status: implemented
verification:
  - manual
  - ui
---

## Statement

The system **shall** place labels in priority order (selection, arc ends,
islands, then directories by depth), skipping any label that would overlap one
already placed, lie off screen, or, in walk mode, cover any part of what the
walker is holding, and **shall** place at most 160 labels.

## Rationale

Greedy placement by priority keeps the most important names and never lets
labels cover one another.

## Acceptance criteria

1. No two visible labels overlap.
2. A selection's own label is never dropped in favour of a region label.
3. In walk mode no label is placed over the hands or what they hold, and a label
   anywhere else on screen is not dropped because of them: what they cover is
   their outline, not a box around them.
