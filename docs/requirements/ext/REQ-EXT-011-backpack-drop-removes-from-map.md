---
id: REQ-EXT-011
uuid: 687a27f0-88ac-432a-a36c-307a6c1717f3
title: Finding dropped in the panel leaves the map
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

Removing an entry from the Backpack view **shall** send the backpack without
that entry to the server (`PUT /api/backpack`), so that it is removed from the
map's backpack as well.

## Rationale

The catch is one; the server holds the session's copy and announces the change
to the map.

## Acceptance criteria

1. After an entry is dropped in the panel, `GET /api/session` returns a backpack
   without it and the view no longer lists it.
2. Taking a finding out of the panel takes it out of the map's backpack.
