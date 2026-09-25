---
id: REQ-LANG-019
uuid: ca4f706f-d421-4d16-ade4-28accea7396f
title: User exclude globs
scope: lang
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M9
verification:
  - unit
---

## Statement

The system **shall** omit every file whose relative path, or any segment of
whose relative path, matches one of the user's `exclude` glob patterns.

## Rationale

Users need to hide generated or irrelevant trees that are not ignored by git.

## Acceptance criteria

1. With the exclude pattern `gen`, the file `gen/skip.go` is not listed.
2. Files not matching any pattern are listed.

## Notes

Where exclude patterns come from and how they combine is scope `cfg`.
