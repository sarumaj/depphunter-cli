---
id: REQ-PERF-005
uuid: ee237615-8601-4dbf-be7b-68dabc79a481
title: Darts dropped on relayout
scope: perf
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - manual
  - inspection
---

## Statement

The walker **shall** drop every dart in flight when a relayout replaces the
boxes it was aimed at.

## Rationale

A dart aimed at a box of the old layout would hit a box that no longer exists.

## Acceptance criteria

1. A live update while a dart is in flight removes the dart without an error.
