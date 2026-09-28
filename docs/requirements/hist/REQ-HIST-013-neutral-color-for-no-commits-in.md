---
id: REQ-HIST-013
title: Neutral color for no commits in range
scope: hist
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

In a history mode a file or district without changes in the selected range
**shall** be drawn in a dedicated neutral color, labeled "no commits in range"
(or "not committed" in Last change mode) in the legend.

## Rationale

No data must not be confused with the low end of the scale.

## Acceptance criteria

1. Moving the slider past a file's last change draws it in the neutral color
   shown in the legend.
