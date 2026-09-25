---
id: REQ-TOOL-056
uuid: 74cd348d-2c43-484c-a727-e53f71556c88
title: Q cycles the carried row
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

`Q` **shall** take the next carried tool into the left hand, and after the last
of them leave the left hand empty.

## Rationale

An empty hand belongs in the ring: putting the jet away is how a walker comes
down.

## Acceptance criteria

1. `Q` pressed four times from empty comes back to empty.
