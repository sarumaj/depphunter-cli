---
id: REQ-TOOL-010
title: Hand model loaded once, cloned per hand
scope: tool
type: non-functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The hand model **shall** be loaded once per page and every hand drawn **shall**
be a clone sharing its geometry.

## Rationale

A second hand should cost a skeleton and nothing else.

## Acceptance criteria

1. Two hands on screen trigger a single fetch of `hand.glb`.
2. A left hand is the right hand mirrored.
