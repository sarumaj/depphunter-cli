---
id: REQ-TOOL-037
uuid: 1474c7d5-f973-421e-a2f8-b11785a216aa
title: No floating parts on a tool
scope: tool
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M26
verification:
  - ui
---

## Statement

Every part of a tool **shall** touch another part of it, so that all parts
connect to the part held; a part that has to reach something **shall** be built
between its two ends.

## Rationale

A part placed at an angle with a length of its own arrives wherever that length
runs out, which is how a thruster came to hang in the air.

## Acceptance criteria

1. No tool carries a part that touches nothing else on it (M26 acceptance).
