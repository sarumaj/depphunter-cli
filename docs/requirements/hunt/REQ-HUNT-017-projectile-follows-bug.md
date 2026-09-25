---
id: REQ-HUNT-017
uuid: 703bc6cb-3fc6-4bc4-bb53-41a8a8cc18f4
title: Projectiles follow a walking bug
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M13
verification:
  - e2e
---

## Statement

A projectile thrown at a bug **shall** track the bug's current position for the
whole of its flight, so that it arrives where the bug is when it lands.

## Rationale

Bugs keep walking while the projectile flies.

## Acceptance criteria

1. A shot at a moving bug lands on the bug and catches it.
