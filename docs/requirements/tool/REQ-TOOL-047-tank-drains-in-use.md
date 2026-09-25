---
id: REQ-TOOL-047
uuid: 180e69fa-59bf-427d-8724-ca156d1a78d6
title: Jet and skimmers run on a tank
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

The jet backpack and the water skimmers **shall** each have a tank that drains
only while the tool is doing its work (the jet holding the walker off the
ground, the skimmers with only water under them) and refills whenever it is not.

## Rationale

Neither is a way of getting everywhere.

## Acceptance criteria

1. The jet runs out and fills up again.
2. A tank drains only in use: standing on a roof with the jet
   in hand does not drain it.
3. The grapple has no tank.
