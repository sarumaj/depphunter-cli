---
id: REQ-HUNT-029
uuid: 44e397a2-52b8-4b78-b8d1-99a541b835da
title: A finding dropped in the panel leaves the map
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

When a finding is dropped from the backpack in a panel (the editor's backpack
view or the map's side panel), the map's backpack **shall** drop it too and its
bug **shall** return to its lap.

## Rationale

The backpack is one catch shared by every client of the session.

## Acceptance criteria

1. Taking a finding out of the editor panel's backpack takes it out of the map's
  .
2. Its bug walks again on the map.
