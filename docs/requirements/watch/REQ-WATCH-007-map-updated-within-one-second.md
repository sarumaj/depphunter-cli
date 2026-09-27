---
id: REQ-WATCH-007
title: Map updated within one second
scope: watch
type: non-functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

In `--watch` mode, editing a file of a warm-cached project **shall** update the
open map within one second, without losing the view state.

## Rationale

The 300 ms debounce and the incremental re-analysis
leave room for delivery and redraw.

## Acceptance criteria

1. Saving a file in this repository with `--watch` updates the open map in under
   one second, with expansion and selection unchanged.
