---
id: REQ-WALK-005
title: Jump height
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

`Space` **shall** make a walker standing on the ground jump. A jump **shall**
clear a curb and a terrace wall and **shall not** clear a tree.

## Rationale

At the map's scale the earlier jump topped out two and a half stories up, a
walker clearing a tree. The current impulse (3.2 units/s at a gravity of 13)
tops out about 0.4 units up, half a story of 0.84.

## Acceptance criteria

1. A walker can jump onto a terrace from the street beside it.
2. A walker cannot jump over a tree; the trunk still stops them.
3. A jump costs wind (REQ-WALK-038).
