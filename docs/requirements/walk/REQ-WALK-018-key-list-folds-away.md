---
id: REQ-WALK-018
uuid: 024f9582-3e37-493d-9ee0-b5bf0837f44c
title: Key list folds away while moving
scope: walk
type: functional
priority: should
status: implemented
verification:
  - manual
---

## Statement

The walk HUD's key list **should** fold away 2.5 s after the walker starts
moving and **should** come back after 6 s without movement.

## Rationale

The list is needed at the start and in the way afterwards.

## Acceptance criteria

1. Walking for a few seconds hides the key list.
2. Standing still for several seconds shows it again.
