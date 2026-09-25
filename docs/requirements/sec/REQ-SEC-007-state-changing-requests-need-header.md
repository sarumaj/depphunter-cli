---
id: REQ-SEC-007
uuid: 9399637a-ac49-4c2b-bee4-27af6a69973f
title: State-changing requests need a custom header
scope: sec
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M3
verification:
  - integration
---

## Statement

The server **shall** answer 403 to every request with a method other than GET
that does not carry the header `X-Depphunter-Request: 1`.

## Rationale

Cross-site pages cannot set a custom header without a CORS preflight, which the
server never grants, so the header defeats cross-site request forgery of
`/api/open`, `/api/settings` and the other writes.

## Acceptance criteria

1. `POST /api/open` with the cookie but without the header answers 403.
2. `POST /api/settings` with the cookie but without the header answers 403.
3. In embed mode a write without the header also answers 403.
