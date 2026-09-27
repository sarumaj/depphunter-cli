---
id: REQ-MAP-002
title: Expanded directory drawn as a nested terrace
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The system **shall** draw an expanded directory as a terrace (a raised plateau
of fixed thickness) standing on its parent's terrace, with the directory's
visible children placed on its top.

## Rationale

Nesting terraces make the directory hierarchy visible as height and containment
without any lines.

## Acceptance criteria

1. An expanded directory has a box of kind `terrace` whose base is the top of
   its parent's terrace.
2. Every visible child of the directory lies within the terrace's footprint, on
   its top.
