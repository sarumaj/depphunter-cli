---
id: REQ-WALK-023
uuid: 7b0c0406-530a-4143-8ab9-de75443a4229
title: Map shortcuts suspended in walk mode
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

In walk mode the UI **shall not** offer expanding or collapsing nodes, and the
keys walk mode owns - including `E`, `Q` and `Enter` - **shall not** reach the
map's own shortcuts.

## Rationale

Expanding or collapsing rebuilds the whole city around the walker, and
collapsing a directory folds its buildings into a district block, which looked
like buildings vanishing.

## Acceptance criteria

1. Pressing `E`, `Q` or `Enter` in walk mode neither rotates the map nor expands
   or collapses a node.
2. The map-only toolbar controls are hidden in walk mode.

## Notes

What `E`, `Q` and `Enter` do in the street is scope `tool`/`hunt`.
