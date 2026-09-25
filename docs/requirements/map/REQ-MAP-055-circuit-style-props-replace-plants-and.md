---
id: REQ-MAP-055
uuid: 0f202a15-af10-4a7c-99af-bb031da5e330
title: Circuit style props replace plants and lamps
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M14
verification:
  - manual
---

## Statement

In the circuit style, the system **shall** place capacitors and other
through-hole parts where the city has trees, surface-mount resistors where it
has bushes, and LEDs, glowing, where it has street lamps.

## Rationale

The props change with the style; their places do not.

## Acceptance criteria

1. A circuit map shows capacitors and resistors along the shores and LEDs along
   the terrace edges.
