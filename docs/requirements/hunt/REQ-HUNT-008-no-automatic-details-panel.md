---
id: REQ-HUNT-008
uuid: e753c679-5fec-4210-abbf-12881c939e96
title: Details panel never opens on its own
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

While walking, the system **shall not** open the details panel except in
response to a second hit on a tagged module, `Enter`, or a catch; tagging a
module for the first time **shall not** open it.

## Rationale

The panel covers the reticle and cannot be reached with the pointer locked, so
opening it unasked interrupts the walk.

## Acceptance criteria

1. Tagging a new module selects it without opening the side panel.
