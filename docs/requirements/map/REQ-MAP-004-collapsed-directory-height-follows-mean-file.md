---
id: REQ-MAP-004
uuid: 0e13c98f-acd8-4c98-b499-0d0cb1abca87
title: Collapsed directory height follows mean file size
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The system **shall** draw a collapsed directory's district block with a height
given by the mean drawn size of the visible files beneath it, on the same height
scale as buildings.

## Rationale

Height carries size everywhere on the map, so a district is as tall as its
average file would be.

## Acceptance criteria

1. A district whose only file has 500 counted lines is as tall as a building for
   a 500-line file.
2. A district over files with a larger mean size is taller.

## Notes

Fixed: `computeVisibility` (filter.js) now also totals `totalBulk`, the drawn
size of the visible files (lines, or bytes converted for unread files), which
layout.js divides by the file count. It had carried only `fileCount` and
`totalLoc`, so every district was drawn at the floor height (0.2).
`web/uitest/layout.test.mjs` checks both acceptance criteria.
