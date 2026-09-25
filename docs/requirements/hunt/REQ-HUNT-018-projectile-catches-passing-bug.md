---
id: REQ-HUNT-018
uuid: 33d8da66-6da3-45bc-91ba-dc765ac1a708
title: Projectiles catch bugs they pass through
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

A projectile from a tool that catches bugs **shall** catch any uncaught bug it
passes within catching distance of while in free flight, whether or not that bug
was aimed at.

## Rationale

Anything thrown should behave physically along its whole path.

## Acceptance criteria

1. A missed shot whose path crosses another bug catches that bug.
2. A tracking dart, which does not catch bugs, passes through them.
