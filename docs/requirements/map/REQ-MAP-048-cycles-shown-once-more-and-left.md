---
id: REQ-MAP-048
uuid: 9bb0323b-bd10-43ec-9c41-2f016eb15d57
title: Cycles shown once more and left closed
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

A dependency-tree row for a node that is already open further up its own branch
**shall** be shown once more, marked as a repeat, and **shall not** be openable.

## Rationale

Lock files contain cycles; a tree that followed one would not end.

## Acceptance criteria

1. A package that depends on something that depends back on it can be opened
   down to its repeat, which is marked and cannot be opened further.
