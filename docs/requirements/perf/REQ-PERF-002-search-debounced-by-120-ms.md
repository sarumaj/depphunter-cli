---
id: REQ-PERF-002
uuid: 87406726-a0d8-44a5-84ea-6fcf573d681e
title: Search debounced by 120 ms
scope: perf
type: non-functional
priority: must
status: implemented
verification:
  - manual
  - inspection
---

## Statement

The UI **shall** run a search only once typing has paused for 120 ms.

## Rationale

The index can hold 100,000 entries, and ranking them on every keystroke makes
typing lag.

## Acceptance criteria

1. Typing a query quickly runs one search after the last key.
