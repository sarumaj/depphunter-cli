---
id: REQ-WALK-030
uuid: 2b3bb94f-c74d-4ca4-98d0-a904ac8642a7
title: Death ends the walk and keeps the backpack
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M20
verification:
  - ui
  - manual
---

## Statement

When the walker's health reaches zero the walk **shall** end and the isometric
map **shall** return with the backpack intact, and the next walk **shall** start
at full health.

## Rationale

The backpack is what the session is for; dying loses the walk only.

## Acceptance criteria

1. After dying, the backpack holds everything caught before.
2. Walking in again shows full health.
