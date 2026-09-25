---
id: REQ-MAP-054
uuid: 1459412d-b269-4519-b5bd-6215ab2a8ab5
title: Circuit style draws copper streets and board
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

In the circuit style, the system **shall** draw copper traces down the middle of
every street with vias along them, a hatched ground pour on open board, solder
pads inside a silkscreen outline around every part, and the board's layered edge
on terrace sides.

## Rationale

A board's streets carry copper where a city's carry asphalt.

## Acceptance criteria

1. The board's streets carry copper traces where a city's carry asphalt.
2. Terrace sides show the layered board edge.
