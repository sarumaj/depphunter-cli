---
id: REQ-LANG-029
uuid: 6f087598-abb9-4a34-bdb4-3fc7476887c6
title: Cold analysis time
scope: lang
type: non-functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md §6
verification:
  - e2e
---

## Statement

A cold analysis (empty cache) of a project of 10 000 files **shall** complete in
less than 5 seconds.

## Rationale

The tool is meant to answer questions about an unfamiliar repository within a
minute, and the map is not shown before analysis ends.

## Acceptance criteria

1. With an empty cache, the analysis of a 10 000-file project logs a duration
   below 5 s.

## Notes

Measured on 2026-09-25 on a 4-CPU container: a synthetic project of 10 000 Go,
TypeScript and Python files of about 4 KB each (40 MB) took 8.3 s cold. The
target is not met on that machine; no automated benchmark exists.
