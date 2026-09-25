---
id: REQ-EXT-007
uuid: 675cce99-e803-4ce0-abcc-1ed9ca6bfec8
title: Row picked in the panel selects the building
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md The panel beside the code
verification:
  - extension
  - e2e
---

## Statement

Picking a row in the Dependencies view **shall** send that node as the selection
to the server (`POST /api/selection`), so that the map selects the corresponding
building.

## Rationale

Picking a row is the same act as clicking a building; the server holds the
selection while the map is open.

## Acceptance criteria

1. After a row is picked, `GET /api/session` reports that row's node id as
   `selected`.
2. Clicking a package in the panel selects the same package on the map.
