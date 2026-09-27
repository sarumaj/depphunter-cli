---
id: REQ-MAP-047
title: Arrow keys open and close tree rows
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

On a focused dependency-tree row with children, the UI **shall** open the row on
`ArrowRight` and close it on `ArrowLeft`.

## Rationale

A tree is operated with the arrow keys everywhere else.

## Acceptance criteria

1. Focusing a closed row and pressing `ArrowRight` opens it; `ArrowLeft` closes
   it and everything opened below it.
