---
id: REQ-HUNT-019
title: HUD counts the bugs
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

While bugs are on the map, the HUD **shall** show how many have been caught out
of how many there are, so that what is left can be read off it.

## Rationale

The hunt needs a measure of progress.

## Acceptance criteria

1. The counter reads caught and total and updates on every catch.
2. With no bugs on the map the counter is hidden.
