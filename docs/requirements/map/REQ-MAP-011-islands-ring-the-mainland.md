---
id: REQ-MAP-011
uuid: c9cbca0f-f7a4-4f30-9455-ea339595448d
title: Islands ring the mainland
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §2
  - docs/REQUIREMENTS.md M7
verification:
  - ui
  - manual
---

## Statement

The system **shall** place the ecosystem islands in rings around the mainland,
in order of how many imports they receive, most imported first; each island
**shall** go to the side of the current ring whose row would be least full, and
when no side has room a new ring **shall** start outside everything placed so
far.

## Rationale

A large dependency set previously produced one long row north of the mainland; a
ring keeps every island close and the most used ones nearest.

## Acceptance criteria

1. With four islands of equal size, one is placed on each side of the mainland.
2. The most imported ecosystem is in the innermost ring.
3. Islands that do not fit on any side of a ring are placed in a further ring
   outside it, and no two islands overlap.
