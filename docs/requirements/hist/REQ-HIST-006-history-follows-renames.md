---
id: REQ-HIST-006
uuid: b63b1b21-2d2b-4089-96fa-64acb7e11095
title: History follows renames
scope: hist
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The history reader **shall** detect renames (`-M`) and file the changes made
under an earlier name under the path the file has at HEAD, including renames
written as `dir/{old => new}/file`.

## Rationale

Without it a renamed file appears to have been created at the rename, losing its
age and authorship.

## Acceptance criteria

1. `src/{a => b}/x.go` splits into `src/a/x.go` and `src/b/x.go`; `{ =>
   sub}/x.go` into `x.go` and `sub/x.go`.
2. Changes to a file before its rename are counted for its current path.

## Notes

`--follow` works for single files only, so renames are detected with `-M` over
the whole log instead.
