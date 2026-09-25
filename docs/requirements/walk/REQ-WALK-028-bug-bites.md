---
id: REQ-WALK-028
uuid: 5e60d2ba-dc5c-469d-847f-697e994460ed
title: Bug bites
scope: walk
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

A bug within a stride (0.75 units) of the walker **shall** bite, no more than
once every 1.1 s across all bugs, for damage that increases with the finding's
severity; from full base health it **shall** take at least three bites of any
severity to kill the walker.

## Rationale

Being bitten is a reason to deal with the bug rather than an instant loss; a
swarm is dangerous by being hard to escape.

## Acceptance criteria

1. A critical bite costs more than a high one, which costs more than a medium
   one, which costs more than an informational one.
2. An unrecognized severity costs the same as `unknown`.
3. From full base health, killing the walker takes at least three critical
   bites.

## Notes

A bite costs 34 for critical and 4 for info (the severity SARIF `note` maps
to), a ratio of 8.5; against `low` (7) it is about 5.
