---
id: REQ-HUNT-016
title: Caught findings go into the backpack
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

A caught bug's finding **shall** be added to the backpack, at most once per
finding.

## Rationale

The point of catching a finding is to come back to it.

## Acceptance criteria

1. Catching a bug in walk mode makes its finding appear in the backpack.
2. Catching the same finding again does not add a second entry.

## Notes

The side panel's `+` and the findings list (REQ-HUNT-050) catch through the same
function (`catchFinding` in web/static/backpack.js), so an entry reads the same
however it was caught.
