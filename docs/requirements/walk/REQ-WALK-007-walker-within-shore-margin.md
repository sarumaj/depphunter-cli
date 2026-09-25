---
id: REQ-WALK-007
uuid: 8dee6ff5-73dd-4904-bf45-e6f3b5c4df06
title: Walker stays within 3 units of the outermost shore
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M8
verification:
  - manual
---

## Statement

The walker's position **shall** be clamped to the bounding rectangle of the
layout extended by 3 units on every side.

## Rationale

The view stays with the map: walking or flying cannot leave it behind.

## Acceptance criteria

1. Walking or flying straight out to sea stops 3 units beyond the outermost
   shore.
2. A relayout that shrinks the map moves the walker back inside the margin.
