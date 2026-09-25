---
id: REQ-MAP-030
uuid: e2521c1a-5d1f-4dca-86fa-5a7422943956
title: Aggregate statistics for directories and packages
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
verification:
  - manual
  - ui
---

## Statement

The side panel **shall** show aggregate statistics for the selected node: for a
directory its files, lines, sub-directories and language mix by lines; for a
package its version (and requested specifier), importing files, ecosystem and
index; for an ecosystem its package count.

## Rationale

A directory or package has no source to show; its aggregates are what describe
it.

## Acceptance criteria

1. Selecting a directory shows its file count, line count and a language
   breakdown.
2. Selecting a package shows its version and number of importing files.
