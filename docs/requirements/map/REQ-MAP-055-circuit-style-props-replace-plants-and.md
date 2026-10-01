---
id: REQ-MAP-055
title: Circuit style props replace plants and lamps
scope: map
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

In the circuit style, the system **shall** place through-hole parts where the
city has trees, surface-mount parts where it has bushes, and LEDs, glowing,
where it has street lamps. The through-hole parts **shall** be of eight kinds:
capacitors (a tall and a squat electrolytic, a ceramic disc, a film box),
transistors (a TO-92 and a TO-220 with its tab) and inductors (a wound drum and
a toroid). The surface-mount parts **shall** be of four: a resistor, a ceramic
capacitor, a SOT-23 transistor and a shielded inductor. Each spot's kind is
picked from its position, so a map dresses the same every time.

## Rationale

The props change with the style; their places do not. A board of three part
shapes, each the same everywhere, read as a pattern rather than as a board.

## Acceptance criteria

1. A circuit map shows capacitors, transistors, inductors and surface-mount
   parts along the shores and in the parks, and LEDs along the terrace edges.
2. Every kind of part is drawn somewhere on a board with enough of them.
