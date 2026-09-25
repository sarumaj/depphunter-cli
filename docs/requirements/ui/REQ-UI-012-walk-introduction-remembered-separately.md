---
id: REQ-UI-012
uuid: 242cb600-085d-4069-92a9-2ce74b62369d
title: Walk introduction remembered separately
scope: ui
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

The UI **shall** remember the walk-mode introduction separately from the map's,
so that seeing either does not count as seeing the other.

## Rationale

They are two different things to be introduced to, at two different moments.

## Acceptance criteria

1. After finishing the map's introduction, a first walk still opens the walk
   introduction.
