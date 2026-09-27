---
id: REQ-HUNT-017
title: Projectiles follow a walking bug
scope: hunt
type: functional
priority: must
status: implemented
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
