---
id: REQ-TOOL-057
uuid: 1c740a9b-cee4-4f41-8052-7391940a497a
title: E cycles the hunting hand
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M31
  - docs/REQUIREMENTS.md M10
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

1. `E` walks the hunt's row round (M31 acceptance).

## Notes

This key was `T` before M31.
