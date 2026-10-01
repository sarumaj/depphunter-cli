---
id: REQ-WALK-023
title: Map shortcuts suspended in walk mode
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

In walk mode the UI **shall not** offer expanding or collapsing single nodes,
and the keys walk mode owns - including `E`, `Q` and `Enter` - **shall not**
reach the map's own shortcuts. Stepping the whole map's depth **shall** stay
offered: by `+` and `-` (REQ-MAP-064) and by the toolbar's depth buttons, which
**shall** stay visible in walk mode.

## Rationale

Expanding or collapsing rebuilds the whole city around the walker, and
collapsing a directory folds its buildings into a district block, which looked
like buildings vanishing. Stepping the depth is asked for on purpose and fades
the change in (REQ-MAP-064); hiding its buttons in the street left the keys as
its only way, which nothing on screen told the walker about.

## Acceptance criteria

1. Pressing `E`, `Q` or `Enter` in walk mode neither rotates the map nor expands
   or collapses a node.
2. The map-only toolbar controls - rotate, fit, reset - are hidden in walk
   mode; the depth buttons and their label are not, and they step the depth.

## Notes

What `E`, `Q` and `Enter` do in the street is scope `tool`/`hunt`.
