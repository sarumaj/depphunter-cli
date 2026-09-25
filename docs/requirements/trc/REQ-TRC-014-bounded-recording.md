---
id: REQ-TRC-014
uuid: 5f3d90b2-ff8f-4f49-ae85-97bbacaa840a
title: Bounded recording
scope: trc
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M18
verification:
  - unit
---

## Statement

The report **shall** keep complete counts of every question but **shall** keep
per-question detail for at most 20 000 questions, counting the rest as dropped
and saying so.

## Rationale

`-1` over a large lock file asks hundreds of thousands of times, and a report
nobody can open explains nothing.

## Acceptance criteria

1. After more than 20 000 questions the totals count all of them, 20 000 entries
   are kept and the report states how many were dropped.

## Notes

No automated test exercises the cap.
