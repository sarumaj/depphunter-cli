---
id: REQ-HIST-013
uuid: 2b2bd05d-d8b2-47bb-b9ce-d24e5d9fe3f7
title: Neutral color for no commits in range
scope: hist
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M5
verification:
  - manual
---

## Statement

In a history mode a file or district without changes in the selected range
**shall** be drawn in a dedicated neutral color, labelled "no commits in range"
(or "not committed" in Last change mode) in the legend.

## Rationale

No data must not be confused with the low end of the scale.

## Acceptance criteria

1. Moving the slider past a file's last change draws it in the neutral color
   shown in the legend.
