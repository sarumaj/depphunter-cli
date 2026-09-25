---
id: REQ-TOOL-051
uuid: 919e03c9-c15d-4fc8-950e-f8b1d4ecf71b
title: Projectiles visible in flight
scope: tool
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M25
verification:
  - manual
---

## Statement

Every projectile **shall** be on screen long enough to be seen; the nail
**shall** fly slower (55 units per second) and larger than before and be drawn
with a streak behind it.

## Rationale

A nail a centimeter across at ninety units a second crossed its own reach in six
frames.

## Acceptance criteria

1. A nail is visible in flight (M25 acceptance).
