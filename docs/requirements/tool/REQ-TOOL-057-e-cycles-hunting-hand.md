---
id: REQ-TOOL-057
title: E cycles the hunting hand
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

`E` **shall** take the next primary tool into the right hand, cycling through
all primary tools and never leaving the hand empty.

## Rationale

There is always something to hunt with; `E` is within reach of the hand on `W`,
`A`, `S`, `D`.

## Acceptance criteria

1. `E` walks the hunt's row round.

## Notes

`E` sits next to the movement keys, where the left hand already is.
