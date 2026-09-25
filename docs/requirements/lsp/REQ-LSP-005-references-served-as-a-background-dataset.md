---
id: REQ-LSP-005
uuid: f79b50d1-2dcd-4ba3-8a06-817e6b757767
title: References served as a background dataset
scope: lsp
type: functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

The server **shall** compute the references in the background after the map is
served and serve them at `GET /api/references` as `{edges, servers, partial}`
with the same status codes as the history (202 while computing, 204 when none),
announcing a `references` event when they change.

## Rationale

Like the history, references take long and must not delay the map.

## Acceptance criteria

1. With `--lsp`, `/api/references` answers 202 until the servers finish, then
   200 with the edges.
2. A `references` event is emitted when the edges become available.
