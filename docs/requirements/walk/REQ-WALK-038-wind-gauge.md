---
id: REQ-WALK-038
uuid: 32189637-7db0-4855-ae01-c32b17633f17
title: Wind gauge for running and jumping
scope: walk
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The HUD **shall** show a wind gauge beside the health bar. Running **shall**
drain it in 9 s, each jump **shall** take 16 % of it, it **shall** refill over 7
s while the walker walks or stands, and flying **shall** cost none of it.

## Rationale

Free sprinting made running the only way anybody moved; a gauge makes a dash a
decision.

## Acceptance criteria

1. One second of running leaves the walker able to run, with less wind.
2. A flat-out sprint runs the gauge out.
3. A rested walker manages between 4 and 12 jumps in a row.
4. Standing fills the gauge back to full and no further.
