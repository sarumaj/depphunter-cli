---
id: REQ-HUNT-015
uuid: d28cdc31-06fb-4808-8b92-0bf180ce4405
title: Catching a bug opens its finding
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

When a bug is caught in walk mode, the system **shall** select the bug's
building and open its details with the caught finding expanded, as a second hit
on a building opens its details, once the catch animation has finished.

## Rationale

Catching a bug reads out what it carried. The panel waits for the catch
animation so that it does not hide it.

## Acceptance criteria

1. Catching a bug opens the same finding the side panel lists for its building
  .
2. The panel opens after the catch animation, not on the frame of the catch.
