---
id: REQ-HUNT-012
uuid: 7fd14c1c-4ee2-44ed-aa54-03647055d21c
title: Severity carried by the bug shape
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

A bug's shape **shall** depend on its severity: a critical finding **shall**
walk as a caterpillar that never flies, high and medium findings as a beetle,
and low, info and unknown findings as a mite.

## Rationale

A color says nothing in a crowd, in the dark or from behind; a shape does. The
worst finding is the one that has to be walked up to.

## Acceptance criteria

1. A critical finding's bug uses the caterpillar geometry and is never placed on
   an air lap.
2. High and medium findings use the beetle; low and info use the mite.
3. A critical finding is visibly a different animal from a note.
