---
id: REQ-SEC-003
uuid: 113796ea-2871-4689-8920-36eefce09076
title: Token exchanged for a cookie
scope: sec
type: non-functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

On a request carrying the correct token in the query string, the server
**shall** set an `HttpOnly`, `SameSite=Strict` cookie holding the token, under a
name particular to that server, and redirect to the same URL without the token.

## Rationale

The token leaves the address bar and never reaches a link or a log. A per-server
cookie name keeps two maps open in one browser from overwriting each other's
cookie.

## Acceptance criteria

1. Opening the printed URL redirects to the map, which then loads.
2. After logging in to two servers with one cookie jar, both answer `/api/graph`
   with 200.
