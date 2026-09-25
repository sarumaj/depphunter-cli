---
id: REQ-TRC-010
uuid: b4b641d5-7b29-47ca-a740-ffc525c25cdd
title: Explain writes the digest to the log
scope: trc
type: interface
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

With `--explain` (or `explain: true`, or `DEPPHUNTER_EXPLAIN`) the system
**shall** write the text digest of the report to the log after each analysis;
under `--watch` only after a re-analysis that changed the map.

## Rationale

The log is where the editor's output channel reads it.

## Acceptance criteria

1. `--explain` writes the digest after the initial analysis.
2. A project configuration may set `explain`.
