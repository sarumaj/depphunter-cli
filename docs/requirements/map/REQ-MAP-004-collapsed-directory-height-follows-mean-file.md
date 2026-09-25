---
id: REQ-MAP-004
uuid: 0e13c98f-acd8-4c98-b499-0d0cb1abca87
title: Collapsed directory height follows mean file size
scope: map
type: functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md §2
  - docs/REQUIREMENTS.md M32
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

Partial: layout.js computes the height from `totalBulk` of the entry
`computeVisibility` (filter.js) returns for the directory, but that entry only
carries `fileCount` and `totalLoc`. The height is therefore computed from `NaN`,
which falls back to 0, and every district is drawn at the floor height (0.2).
The regression came with M32 (drawn size from bytes). Colors by size use the
model's `totalBulk` and are not affected.
