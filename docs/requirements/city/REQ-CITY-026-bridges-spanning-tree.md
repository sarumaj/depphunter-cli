---
id: REQ-CITY-026
uuid: 24885505-0fa5-4692-ad94-3029107f288f
title: Bridges link every island
scope: city
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
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
