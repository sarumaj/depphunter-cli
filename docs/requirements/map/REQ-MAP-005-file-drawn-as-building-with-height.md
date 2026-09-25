---
id: REQ-MAP-005
uuid: ff543e2f-a0c1-49ac-81cf-999a61e906a4
title: File drawn as building with height from lines
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §2
verification:
  - ui
  - manual
---

## Statement

The system **shall** draw a collapsed file as a building of fixed footprint (1
unit square) whose height is its drawn size on the selected height scale, the
tallest building being 10 units above a 0.2-unit floor.

## Rationale

A fixed footprint keeps buildings comparable; height alone carries size, which
is the one quantity a skyline shows best.

## Acceptance criteria

1. Every file box has kind `building` and a 1 x 1 footprint.
2. The file with the most counted lines is 10.2 units tall; an empty file is 0.2
   units tall.
