---
id: REQ-CITY-026
title: Bridges link every island
scope: city
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

Every island **shall** be linked by exactly one bridge, the bridges forming a
spanning tree rooted at the mainland (the largest shore), each island joined to
the nearest shore already reachable.

## Rationale

Every island must be reachable on foot.

## Acceptance criteria

1. A map with a mainland and three islands has three bridges, and every island
   is on the end of one.
