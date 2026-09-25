---
id: REQ-MAP-046
uuid: 775c2ba9-e12e-4aeb-85f4-432eaece05e3
title: Open tree branches survive a live update
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The side panel **shall** reopen, after a live update redraws it, every
dependency-tree branch that was open before, identified by direction and path
from the selected node.

## Rationale

A redraw must not undo what the reader opened.

## Acceptance criteria

1. With a branch open two levels deep, a live update redraws the panel with the
   same branch open.
