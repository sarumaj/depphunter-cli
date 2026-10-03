---
id: REQ-TOOL-032
title: H puts both hands' tools down
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
  - unit
---

## Statement

Pressing `H` **shall** put down what both hands hold, and pressing it again
**shall** take the same tools out again. Put down is put down: with the right
hand empty no click uses its tool, as with the left hand when its key puts its
tool away. Pressing the key of the tool already in the right hand **shall** put
that one down on its own, and any tool's key **shall** take it out again.

## Rationale

An empty view is worth having for screenshots and for looking down, and empty
hands are what playing in the parks takes (REQ-WALK-057). Hands that were only
hidden still fired whatever they held, so a click meant for a ball could net a
building.

## Acceptance criteria

1. After `H` no hand is drawn and a click uses no tool; after a second `H` both
   tools are back.
2. The right hand's own key puts its tool down, and pressing it again takes it
   out.
