---
id: REQ-MAP-034
uuid: 7249e9a6-c4df-48c7-a607-d37dd751cc60
title: Filter by path globs
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M2
verification:
  - ui
  - manual
---

## Statement

The UI **shall** filter files by a comma-separated list of gitignore-style
globs, where a plain glob keeps only matching files and a glob prefixed with `!`
hides matching files; `*` and `?` **shall** stay within a path segment, `**`
span segments, and a glob without `/` match any segment.

## Rationale

Globs select parts of a repository by where they are, which languages and
islands cannot.

## Acceptance criteria

1. `src/**` hides every file outside `src/`.
2. `!**/testdata/**` hides every file under a `testdata` directory.
3. `*.go` keeps Go files in any directory.
