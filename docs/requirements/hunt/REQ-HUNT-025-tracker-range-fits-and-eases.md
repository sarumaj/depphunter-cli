---
id: REQ-HUNT-025
uuid: ac63e527-0403-4b26-b419-637b98e23513
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
is nearer than 14 units, and **shall** change by easing over time rather than
jumping.

## Rationale

A four-file and a thousand-file repository are both worth seeing whole, and the
last steps to a bug are the ones worth seeing close up.

## Acceptance criteria

1. The range grows and shrinks smoothly as bugs are caught.
2. The range never goes below 12 or above 400 units.
3. The easing is by elapsed time, so it looks the same at any frame rate.
