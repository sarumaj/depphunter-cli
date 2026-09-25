---
id: REQ-SRV-010
uuid: f3b16e9e-2a32-4646-9876-a326637c76af
title: Shared selection with announcement
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

The server **shall** record the node selected by a `POST /api/selection` request
(`{"id", "origin"}`) and, when the selection changed, announce a `selection`
event carrying the id and the origin on the event stream.

## Rationale

A row picked in the editor panel selects the building on the map and vice versa;
an unchanged selection is not news.

## Acceptance criteria

1. A new selection is announced once with its id and origin.
2. Posting the same selection again announces nothing.
