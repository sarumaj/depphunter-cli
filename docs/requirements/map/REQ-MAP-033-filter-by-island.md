---
id: REQ-MAP-033
title: Filter by island
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The UI **shall** hide and show each ecosystem island, with all its packages,
from a checkbox list in the Filters menu.

## Rationale

Some ecosystems, such as a standard library, are rarely of interest.

## Acceptance criteria

1. Unchecking an island removes it and its packages from the map.
