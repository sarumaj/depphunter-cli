---
id: REQ-TOOL-066
uuid: bf5d1ae7-6c97-4eba-a948-46e1de65d6e2
title: A jump cuts a line only once it is out
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

A line from the grapple gun or the fishing rod that has bitten **shall** pull the
walker along it until it lets go, and a press of `Space` made after it bit
**shall** cut it. `Space` still held from a jump the line was cast in **shall
not** cut it.

## Rationale

Casting mid-jump is how a line is thrown over an edge or across a gap; a jump
key still down from that jump is not a request to let go.

## Acceptance criteria

1. Jump, cast the grapple at a building while holding `Space`: the line bites
   and pulls the walker onto the roof.
2. Pressing `Space` again while being pulled cuts the line.
3. The same holds for the fishing rod.
