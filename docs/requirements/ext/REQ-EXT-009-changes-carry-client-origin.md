---
id: REQ-EXT-009
uuid: 711dccdf-31f2-45aa-a0c7-fa8d54759e6b
title: Changes name the client that made them
scope: ext
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M15
verification:
  - extension
---

## Statement

Every selection and backpack change the extension sends **shall** carry an
`origin` naming this client, and the extension **shall not** re-apply a
selection announcement that carries its own origin.

## Rationale

A client has to tell its own change returning on the event stream from another
client's change.

## Acceptance criteria

1. `POST /api/selection` and `PUT /api/backpack` bodies include an `origin`
   unique to the extension instance.
2. A `selection` event whose origin equals the extension's own causes no reveal.
