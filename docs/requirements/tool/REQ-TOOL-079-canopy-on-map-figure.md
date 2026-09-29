---
id: REQ-TOOL-079
title: The map shows a walker who left under a canopy under one
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

A walker who leaves walk mode under an open canopy **shall** be drawn on the map
as the figure under a canopy, and walking in again to that spot **shall** find
them still under it.

## Rationale

The figure on the map is where the walker is; hanging in the air under a canopy
is where they are.

## Acceptance criteria

1. Leaving under a canopy shows a canopy over the figure; leaving on foot does
   not.
