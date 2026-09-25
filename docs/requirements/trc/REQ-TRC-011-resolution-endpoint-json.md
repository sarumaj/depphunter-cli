---
id: REQ-TRC-011
uuid: fa08a57d-1f59-4396-a3da-8e08fe66bfe6
title: Resolution report as JSON
scope: trc
type: interface
priority: must
status: implemented
verification:
  - integration
---

## Statement

`GET /api/resolution` **shall** serve the whole report of the analysis now
served as JSON, and **shall** answer 404 when no report exists.

## Rationale

The JSON keeps every value whole, for tools and for the editor.

## Acceptance criteria

1. Before an analysis handed over a report the endpoint answers 404.
2. Afterwards it answers 200 with `application/json` holding the settings,
   lookups and totals.
3. An unknown `format` is answered 400.
