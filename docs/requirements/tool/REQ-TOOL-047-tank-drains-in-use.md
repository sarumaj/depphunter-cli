---
id: REQ-TOOL-047
title: The jet backpack runs on a tank
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

The jet backpack **shall** have a tank that drains only while it is holding the
walker off the ground and refills whenever it is not. The swim ring **shall**
have none: a swim is paid for out of the walker's wind (REQ-WALK-062).

## Rationale

The jet is not a way of getting everywhere.

## Acceptance criteria

1. The jet runs out and fills up again.
2. A tank drains only in use: standing on a roof with the jet
   in hand does not drain it.
3. The grapple and the swim ring have no tank.
