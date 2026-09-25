---
id: REQ-MAP-010
uuid: f7f1798c-adfa-4afc-bee9-e35ebf1ae65d
title: Positions derived from the hierarchy only
scope: map
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §2
  - docs/REQUIREMENTS.md M7
verification:
  - ui
  - inspection
---

## Statement

The system **shall** derive every position on the map from the directory
hierarchy, the expansion state and a stable ordering of children (directories
before files, each by name), and **shall not** use a force simulation or any
other non-deterministic placement.

## Rationale

A layout that is the same for the same repository is stable and can be learned;
force layouts move on every run.

## Acceptance criteria

1. Laying out the same model with the same state twice yields identical box
   positions.
2. No force-directed or randomized placement code is used in layout.js.
