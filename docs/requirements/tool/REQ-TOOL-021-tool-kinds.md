---
id: REQ-TOOL-021
uuid: 4d20c04e-d11e-4547-a9f5-7b3f8fbc231b
title: Primary and secondary tools
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M20
verification:
  - ui
---

## Statement

Every tool **shall** be either primary, which tags a module or catches a bug, or
secondary, which carries the walker instead: the rod, the net, the camera, the
bubble wand, the extinguisher, the tracking dart and the nail gun **shall** be
primary, and the grapple gun, the jet backpack and the water skimmers **shall**
be secondary.

## Rationale

The two kinds are for different things: the hunt, and getting about.

## Acceptance criteria

1. There are ten tools, seven primary and three secondary.
2. Every tool has exactly one kind and its own slot.
