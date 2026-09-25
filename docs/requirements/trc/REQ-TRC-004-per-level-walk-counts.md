---
id: REQ-TRC-004
uuid: bd38b6a3-5f3c-45e8-a753-ae3908afc5fb
title: Counts per ecosystem and level
scope: trc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

For each plugin and each level of the transitive walk the report **shall**
record how many packages were asked about, how many answered, how many packages
and edges were added, and how long the level took.

## Rationale

How far the walk got decides what the map contains.

## Acceptance criteria

1. A walk of `direct -> middle -> deep` with `-1` records three levels with
   asked/answered/added of 1/1/1, 1/1/1 and 1/0/0.
