---
id: REQ-MAP-042
uuid: ce4f45f4-df29-4cb6-8512-dce1ed3b928c
title: Labels placed greedily without overlaps
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M7
verification:
  - manual
  - ui
---

## Statement

The system **shall** place labels in priority order (selection, arc ends,
islands, then directories by depth), skipping any label that would overlap one
already placed or lie off screen, and **shall** place at most 160 labels.

## Rationale

Greedy placement by priority keeps the most important names and never lets
labels cover one another.

## Acceptance criteria

1. No two visible labels overlap.
2. A selection's own label is never dropped in favour of a region label.
