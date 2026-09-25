---
id: REQ-EXT-002
uuid: 3a571246-b379-404d-aa97-c3b500e5d7fd
title: Panel follows the map opened last
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md The panel beside the code
verification:
  - extension
---

## Statement

The Dependencies and Backpack views **shall** show the graph and the session of
the server whose map was opened or started most recently, and **shall** be
emptied when that server stops.

## Rationale

One window may map several folders; the map opened last is the one being looked
at.

## Acceptance criteria

1. After a map is opened for a folder, the Dependencies view title names that
   folder and the tree shows its graph.
2. Opening the map of a second folder re-points both views at the second server.
3. Stopping the server the views show empties both views.
