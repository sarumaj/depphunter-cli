---
id: REQ-SRV-011
uuid: ffc4a51d-afab-49b7-9af1-ed6c849579f3
title: Shared backpack with announcement
scope: srv
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M15
verification:
  - integration
  - extension
---

## Statement

The server **shall** replace its copy of the backpack with the items of a `PUT
/api/backpack` request, keeping at most 500 items and rejecting an item without
an id, and **shall** announce a `backpack` event with the item count and the
origin when the content changed.

## Rationale

The browser owns the lasting store, so the server replaces rather than merges: a
merge would resurrect what was just cleared. 500 is what the browser backpack
holds.

## Acceptance criteria

1. An upload of 501 items keeps the first 500.
2. An item without an id is answered 400.
3. Uploading an identical list announces nothing.
