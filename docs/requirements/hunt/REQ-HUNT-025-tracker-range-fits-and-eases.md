---
id: REQ-HUNT-025
title: Tracker range fits the hunt and eases
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
  - manual
---

## Statement

The tracker's range **shall** fit the farthest remaining target within fixed
bounds (12 to 400 units), **shall** close in on the neighborhood when a target
is nearer than 40 units, and **shall** change by easing over time rather than
jumping. Closing in, the dial **shall** grow by up to 85%, most of the way
while the target is still well off (by the square root of how near it has
come), and **shall** draw a bearing from the walker to the nearest bug, ringed.

## Rationale

A four-file and a thousand-file repository are both worth seeing whole, and the
walk to the nearest bug is worth seeing close up. Closing in only inside 14
units, and only gradually, the sweep helped with the last few steps and not
with finding the way there.

## Acceptance criteria

1. The range grows and shrinks smoothly as bugs are caught.
2. The range never goes below 12 or above 400 units.
3. The easing is by elapsed time, so it looks the same at any frame rate.
4. With a target 30 units off the dial has already grown by more than a third,
   and it grows further the nearer the target comes; with none within 40 it
   does not grow.
