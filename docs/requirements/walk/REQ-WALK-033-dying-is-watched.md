---
id: REQ-WALK-033
uuid: 2dd8ef60-8b4d-452f-b9c7-c6c155851f44
title: Dying is watched
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M21
verification:
  - manual
---

## Statement

When the walker dies, walk mode **shall** stop answering to keys and buttons,
fill the screen with red over about 0.5 s, hold it, and return to the map 1.1 s
after death.

## Rationale

A cut straight to the map reads as a bug rather than as dying.

## Acceptance criteria

1. Dying reddens the screen before the map returns.
2. Keys pressed during the red do nothing.
