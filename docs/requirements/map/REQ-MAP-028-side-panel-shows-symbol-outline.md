---
id: REQ-MAP-028
uuid: dcbb4543-2f3d-4426-b685-49da9e168225
title: Side panel shows symbol outline
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M1
verification:
  - manual
---

## Statement

The side panel **shall** list the symbols of a selected file with their kind and
line, each row selecting that symbol.

## Rationale

An outline is the quickest way into a long file.

## Acceptance criteria

1. A file with symbols shows a Symbols section with one row per symbol and its
   count.
2. Clicking a symbol row selects the symbol.
