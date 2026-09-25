---
id: REQ-SRV-012
uuid: 764918c8-7060-4f20-a60c-d7d18801538e
title: Changes name the client that made them
scope: srv
type: functional
priority: must
status: implemented
verification:
  - integration
  - extension
---

## Statement

Every shared-state change **shall** carry the identifier of the client that made
it, the announcement **shall** carry it back, and a client **shall** ignore
announcements of its own changes.

## Rationale

Without the origin every client would apply its own change a second time, and
two clients watching each other would never settle.

## Acceptance criteria

1. The page sends `origin` with every selection and backpack write, using an
   identifier generated per page load.
2. An announcement whose origin equals the page identifier leaves the page state
   unchanged.
