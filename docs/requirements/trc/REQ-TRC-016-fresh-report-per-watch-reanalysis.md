---
id: REQ-TRC-016
title: A fresh report per re-analysis
scope: trc
type: functional
priority: must
status: implemented
verification:
  - integration
  - inspection
---

## Statement

Under `--watch` each re-analysis **shall** record into a new report and
**shall** publish it at `/api/resolution` in place of the previous one.

## Rationale

A report that accumulated over a morning's editing describes no run in
particular.

## Acceptance criteria

1. After a re-analysis `/api/resolution` serves a report whose generation time
   is that of the re-analysis and whose counts are its own.
