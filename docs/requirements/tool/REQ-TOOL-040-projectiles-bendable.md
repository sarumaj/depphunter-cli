---
id: REQ-TOOL-040
uuid: a4f3c72f-3e2a-473b-881c-7c54156715d2
title: Projectiles follow the planet curve
scope: tool
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M22
verification:
  - inspection
---

## Statement

Every projectile and line **shall** use a bendable material so that it is bent
around the planet like the map.

## Rationale

An unbent projectile flies straight through a curved world.

## Acceptance criteria

1. Every projectile material passes through the scene's bend.
