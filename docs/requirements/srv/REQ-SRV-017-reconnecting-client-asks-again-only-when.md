---
id: REQ-SRV-017
uuid: 21360c23-59c6-4653-b678-7ee9d4c606d2
title: Reconnecting client asks again only when needed
scope: srv
type: functional
priority: must
status: implemented
verification:
  - manual
  - e2e
---

## Statement

The UI **shall** make no request on the first greeting of a connection, and
after a reconnection that did not resume **shall** request the graph again with
`If-None-Match` and the shared session, keeping its model when the server
answers 304.

## Rationale

The entity tag makes asking again free when the answer is the document the page
already holds.

## Acceptance criteria

1. Loading the map makes exactly one `/api/graph` request.
2. A reconnection that missed nothing makes no `/api/graph` request.
3. A reconnection that missed something is answered 304 when the graph did not
   change.
