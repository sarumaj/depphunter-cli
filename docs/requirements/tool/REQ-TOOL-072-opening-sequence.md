---
id: REQ-TOOL-072
title: The canopy takes about a second to open
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

Once thrown, the pilot chute **shall** take 0.3 seconds to drag the canopy out,
and the canopy **shall** then take about a second to open, spreading from the
middle cells outwards and snapping past its full size before it settles, with
its drag growing as it fills; the walker **shall** feel the opening as a jolt. A
canopy that reaches the ground before it has opened **shall** have saved about
as much as it had time to: its landing is judged mostly as the fall it nearly
was.

## Rationale

An opening that takes time is what makes the height it is thrown from matter,
and a parachute thrown at the last moment should not be a way to fall off a
roof for free.

## Acceptance criteria

1. The pilot chute takes 0.3 seconds and the canopy between 0.8 and 1.2 seconds
   more, reaching more than its full size on the way.
2. A canopy thrown into a fast fall decelerates the walker at well over the pull
   of gravity, and the lines give.
3. A canopy thrown two units above the ground is not open when it arrives, and
   the landing costs within a tenth of the walker of what the fall would have.
