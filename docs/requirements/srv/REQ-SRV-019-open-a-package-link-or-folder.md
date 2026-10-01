---
id: REQ-SRV-019
title: Open a package's page, repository or folder
scope: srv
type: functional
priority: should
status: implemented
verification:
  - integration
---

## Statement

`POST /api/browse` with `{"id": "<package id>", "to": "page" | "repository" |
"folder"}` **shall** open the package's `page` or `repository` (REQ-MOD-014),
or the folder REQ-SRV-018 finds. While an event stream opened with
`?opens=links` is connected, it **shall** be handed a `browse` event carrying
the package's `name` and its `url` or `folder`, to it alone, and the answer
**shall** be 202. Otherwise a folder **shall** be shown in the system's file
manager (204) when the server listens on loopback only, and anything else
**shall** answer 501. A package without what was asked for answers 404.

## Rationale

The map in the editor's tab is in a frame that can open neither a window nor a
folder; the extension beside it can. A request names a package and never an
address, so nothing but what the graph says, or what the server found itself,
is ever opened.

## Acceptance criteria

1. With a stream on `?opens=hex,links` connected, a request for a repository
   answers 202 and that stream, and no other, receives the URL.
2. With none connected, a request for a folder starts the file manager on it
   and answers 204, and a request for a page answers 501.
3. An unknown `to` answers 400.
