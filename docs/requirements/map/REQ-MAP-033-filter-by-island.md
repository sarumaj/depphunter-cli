---
id: REQ-MAP-033
uuid: a9eb071d-ba8a-45e9-8a4a-74ae7bbced20
title: Filter by island
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

The UI **shall** hide and show each ecosystem island, with all its packages,
from a checkbox list in the Filters menu.

## Rationale

Some ecosystems, such as a standard library, are rarely of interest.

## Acceptance criteria

1. Unchecking an island removes it and its packages from the map.
