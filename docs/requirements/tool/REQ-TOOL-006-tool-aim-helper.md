---
id: REQ-TOOL-006
uuid: 64b9d022-0053-41e4-a815-132d7c90ecda
title: Each tool has its own aim helper
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
  - manual
---

## Statement

The reticle **shall** be the aim helper of the primary tool in hand; the
tracking dart's aim helper **shall** be a scope reticle.

## Rationale

The crosshair belongs to the dart, not to every tool; a bobber, a hoop, a
viewfinder, a soft ring and a cone fit their tools better.

## Acceptance criteria

1. With the dart in hand the reticle is a scope reticle.
2. Switching to the camera changes the reticle to a viewfinder frame.
3. A secondary tool in the off hand does not change the reticle.
