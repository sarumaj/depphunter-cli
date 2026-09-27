---
id: REQ-PERF-007
title: CSS colors parsed once per pass
scope: perf
type: non-functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The system **shall** parse each distinct CSS color string at most once per
recoloring pass.

## Rationale

Colors arrive as CSS strings, one per box, and every ground vertex reads its
box's again; parsing per box and vertex is too slow.

## Acceptance criteria

1. During one recoloring pass, a color shared by many boxes is parsed once.
