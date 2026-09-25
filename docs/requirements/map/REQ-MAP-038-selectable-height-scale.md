---
id: REQ-MAP-038
uuid: c32cba07-d2da-4086-b0d7-29b1161c43fb
title: Selectable height scale
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M1
verification:
  - ui
  - manual
---

## Statement

The UI **shall** offer linear, square-root (default) and logarithmic height
scales, where the logarithmic scale maps a fraction t of the largest size to
ln(1 + 1000 t) / ln(1001), and **shall** relayout the map when the scale
changes.

## Rationale

Code size is heavily skewed; a compressed scale keeps small files visible next
to large ones.

## Acceptance criteria

1. Switching Height to Lines relayouts with heights proportional to lines.
2. The legend names the current scale.
