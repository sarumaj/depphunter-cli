---
id: REQ-TOOL-048
title: Tank gauge beside the health bar
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

While a tool with a tank is in the off hand, the HUD **shall** show a gauge of
what is left beside the health bar, and **shall** hide it otherwise.

## Rationale

The walker must know how long they can stay up.

## Acceptance criteria

1. Carrying the jet shows the gauge; carrying the grapple or nothing hides it.
