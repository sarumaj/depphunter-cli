---
id: REQ-MAP-032
title: Filter by language
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The UI **shall** hide and show the files of a language from a checkbox list in
the Filters menu and by clicking the language's legend entry (the "Other" entry
acting on every language folded into it).

## Rationale

Hiding languages removes noise such as generated or configuration files.

## Acceptance criteria

1. Unchecking a language removes its buildings from the map.
2. Clicking a legend entry hides that language; clicking it again shows it.
