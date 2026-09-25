---
id: REQ-WATCH-002
uuid: f80f663c-3ec8-4fb1-abfc-756c155165ea
title: Debounced re-analysis
scope: watch
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** re-analyze the project once file-system changes have been
quiet for 300 ms, coalescing a burst of changes into one re-analysis, and
**shall** run another re-analysis afterwards for changes that arrive while one
runs.

## Rationale

An editor save or a `git checkout` produces many events; one analysis per burst
keeps the map current without redundant work.

## Acceptance criteria

1. Several writes within the debounce interval cause exactly one call.
2. A change arriving during a re-analysis causes one further re-analysis after
   it.

## Notes

Two refinements: a stream of changes that
never goes quiet defers the re-analysis by at most ten debounce intervals, and
editor scratch files (swap, backup, lock files) and metadata-only events are
ignored.
