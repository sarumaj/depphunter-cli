---
id: REQ-SUP-032
uuid: 7863a046-4fcf-4c42-887f-08ed1977f338
title: Index answers are cached for a day
scope: sup
type: non-functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** keep every answer an index gave on disk for 24 hours
and reuse it across runs, and **shall** ask again once it has expired.

## Rationale

The same packages are asked about on every run; an index is owed as few requests
as possible.

## Acceptance criteria

1. A second client with the same cache directory answers without a request.
2. An answer older than its time to live is asked for again.
3. A question the index failed to answer is asked again after 5 minutes, not
   before.

## Notes

With `--no-cache` the answers are kept for the current run only.
