---
id: REQ-TOOL-022
uuid: a39943d1-0ecb-4332-8291-0372775c388b
title: Tool kind decided in one place
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

Whether a tool can tag or catch **shall** be decided by a single function from
its kind, which **shall** answer no for every secondary tool and **shall**
determine what the crosshair marks, what a landing shot does and how the slot is
drawn.

## Rationale

One place deciding it keeps the aim, the landing and the HUD consistent.

## Acceptance criteria

1. A secondary tool tags nothing it is pointed at.
2. No secondary tool catches bugs or tags buildings.
