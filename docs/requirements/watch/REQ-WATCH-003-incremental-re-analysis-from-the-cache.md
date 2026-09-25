---
id: REQ-WATCH-003
uuid: bc631d9e-d806-4064-a0ae-9dc80c9f2ad4
title: Incremental re-analysis from the cache
scope: watch
type: functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

A re-analysis in watch mode **shall** re-parse only files whose content changed,
taking all others from the content-hash extraction cache, and **shall** resolve
them against the current manifests.

## Rationale

Only the incremental path makes a sub-second update of a large project possible.

## Acceptance criteria

1. After editing one file, the log reports `updated: 1 files re-parsed`.

## Notes

The extraction cache itself is specified in scope `lang`.
