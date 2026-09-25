---
id: REQ-MAP-043
uuid: 6aecf06c-948d-464a-928f-b34dc1c1ddae
title: Terrace children packed with potpack
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - inspection
---

## Statement

The system **shall** pack a terrace's children into a near-square area with
potpack, each child padded by the sibling gap, in the children's stable order.

## Rationale

A packing library replaces hand-rolled shelf packing; its stable sort keeps the
same tree packing the same way.

## Acceptance criteria

1. No two children of a terrace overlap.
2. Packing the same children twice gives the same positions.
