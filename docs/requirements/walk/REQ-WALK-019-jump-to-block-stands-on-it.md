---
id: REQ-WALK-019
uuid: 8ec9ee83-5f24-48ce-bce7-45f0fc4371e4
title: Jumping to a directory stands the walker on its block
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

When the map brings a node into view while walking (search, breadcrumbs,
dependency lists), the walker **shall** be placed on the node's block, at its
south edge looking across it, if the node is a directory; otherwise beside the
building on its lowest dry side, looking at it.

## Rationale

Standing beside a directory's block put the walker in the street below it,
looking at a wall.

## Acceptance criteria

1. Choosing a directory in the search while walking stands the walker on that
   block.
2. Choosing a file stands the walker in front of its building, facing it.
