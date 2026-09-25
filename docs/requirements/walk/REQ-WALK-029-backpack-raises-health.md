---
id: REQ-WALK-029
uuid: 6bf53ef5-80b7-441c-bc46-35e5070e73cc
title: Backpack raises the health ceiling
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M20
verification:
  - ui
---

## Statement

Every finding in the backpack **shall** raise the walker's maximum health by 8
points (up to 150 points above the base of 100) and **shall** heal the walker by
the same amount.

## Rationale

A walker who has been catching things can stand in a swarm; catching is the only
way to be worth more than one started.

## Acceptance criteria

1. Catching one finding raises the maximum and current health by the same
   amount.
2. A backpack of 10 000 findings raises the maximum by at most 150.
