---
id: REQ-TOOL-050
uuid: db0dcb68-9545-460f-9dd4-f77be9ed3d8d
title: An empty tool stays stopped until it has refilled
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

A tool whose tank has run dry in use **shall** stay stopped until its tank has
refilled to a quarter, and **shall** then work again without being put away and
taken out, whether or not it is in the hand meanwhile.

## Rationale

An empty jet that keeps catching is worse than one that has plainly stopped.

## Acceptance criteria

1. A tank that has run dry does not work while it holds less than a quarter.
2. Once it has refilled to a quarter it works again, still in the hand, and the
   walker is told so.
3. Taking it out again still brings it back, with whatever it holds.
