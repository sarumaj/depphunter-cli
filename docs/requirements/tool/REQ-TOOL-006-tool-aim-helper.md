---
id: REQ-TOOL-006
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

The reticle **shall** be the aim helper of the primary tool in hand. A tool
that throws or fires something is aimed by the path drawn for its shot instead
(REQ-TOOL-082) and **shall** show no reticle; the scope still narrows the view
for the tracking dart.

## Rationale

The crosshair belongs to the dart, not to every tool; a bobber, a hoop, a
viewfinder, a soft ring and a cone fit their tools better.

## Acceptance criteria

1. With the dart, the nail gun, the rod or the bubble wand in hand there is no
   reticle, and the dart's scope still narrows the view.
2. Switching to the camera changes the reticle to a viewfinder frame.
3. A secondary tool in the off hand does not change the reticle.
