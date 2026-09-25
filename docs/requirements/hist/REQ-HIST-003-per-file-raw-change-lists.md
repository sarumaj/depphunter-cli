---
id: REQ-HIST-003
uuid: be85aa7a-fc78-49c3-ad87-6c22e37b45d8
title: Per-file raw change lists
scope: hist
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M5
verification:
  - integration
---

## Statement

The history **shall** record, for every file path, its changes newest first as
arrays `[time, author, added, deleted, commit]` (unix time, author index, lines
added, lines deleted, commit index), with binary changes counting zero lines.

## Rationale

Decision (M5): raw changes are sent instead of server-side aggregates so any
time range can be evaluated in the browser instantly, and the payload stays
small.

## Acceptance criteria

1. A commit adding two lines to `a.go` at time 1000 by the first author appears
   as `[1000,0,2,0,0]`.
