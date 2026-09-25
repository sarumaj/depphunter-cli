---
id: REQ-HUNT-048
uuid: b3ce8059-284a-428f-945e-4504dc43cc93
title: HUD text readable over any street
scope: hunt
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M29
verification:
  - manual
  - inspection
---

## Statement

Every walk-mode HUD element that carries words **shall** be drawn on a
background opaque enough to be legible over any street color.

## Rationale

A panel tinted to a quarter opacity is a panel nobody can read over a pale wall.

## Acceptance criteria

1. Every message in the street is legible over a white wall (M29 acceptance).
