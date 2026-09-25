---
id: REQ-MAP-023
uuid: f4fb0e2d-b0a6-4cdd-ac28-c61db823034c
title: Expand or collapse everything to a depth
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
verification:
  - ui
  - manual
---

## Statement

The UI **shall** expand every directory above a chosen depth and collapse every
directory at or below it, stepping the depth up and down with the toolbar's +
and - buttons and the `+` and `-` keys, between 1 and the deepest directory
level, and **shall** show the current depth.

## Rationale

One step reveals or hides a whole level of the repository at once.

## Acceptance criteria

1. Pressing `+` at depth 2 expands every directory at depth 2 and shows depth 3.
2. The depth never goes below 1 or above the deepest level.

## Notes

The starting depth comes from `expand_depth` (REQ-CFG) or, when unset, the
deepest level that shows at most 600 buildings and districts.
