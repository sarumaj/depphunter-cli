---
id: REQ-MAP-047
uuid: fa65e8b3-e2f3-486a-85a5-020b10edf2a7
title: Arrow keys open and close tree rows
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
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
