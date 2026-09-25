---
id: REQ-TRC-016
uuid: 103bcf7d-9d06-419a-acb4-5447d7dfbc0e
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
