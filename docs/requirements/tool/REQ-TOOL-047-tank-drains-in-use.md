---
id: REQ-TOOL-047
title: Jet and swim ring run on a tank
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

The jet backpack and the swim ring **shall** each have a tank - for the ring,
the swimmer's breath - that drains only while the tool is doing its work (the
jet holding the walker off the ground, the ring with only water under it) and
refills whenever it is not.

## Rationale

Neither is a way of getting everywhere.

## Acceptance criteria

1. The jet runs out and fills up again.
2. A tank drains only in use: standing on a roof with the jet
   in hand does not drain it.
3. The grapple has no tank.
