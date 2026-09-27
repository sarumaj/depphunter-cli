---
id: REQ-SRV-009
title: Shared session endpoint
scope: srv
type: functional
priority: must
status: implemented
verification:
  - integration
  - extension
---

## Statement

The server **shall** answer `GET /api/session` with the shared state of the open
map: the id of the selected node (empty for none) and the backpack items.

## Rationale

The map and the editor side panel are one interface; a client that has just
attached needs the shared state in one round trip.

## Acceptance criteria

1. After a selection and a backpack upload, `GET /api/session` returns both.
2. With nothing selected or caught it returns `{"selected":"","backpack":[]}`.
