---
id: REQ-MAP-005
title: File drawn as building with height from lines
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The system **shall** draw a collapsed file as a building of fixed footprint (a
square as wide as the facade scale, 2.8 units) whose height is its drawn size
on the selected height scale, the tallest building being 10 units above a floor
of one story (0.84 units).

## Rationale

A fixed footprint keeps buildings comparable; height alone carries size, which
is the one quantity a skyline shows best. The footprint grows with the facade
(REQ-CITY-031), so a face holds as many windows as it was laid out with, and
every building is at least a story tall, so that its door fits in it.

## Acceptance criteria

1. Every file box has kind `building` and a 2.8 x 2.8 footprint.
2. The file with the most counted lines is 10.84 units tall; an empty file is
   0.84 units tall.
