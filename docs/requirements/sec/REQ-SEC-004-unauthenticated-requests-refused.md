---
id: REQ-SEC-004
uuid: 23288be2-2b3a-480f-aaca-a0d8f6792bfa
title: Requests without the token refused
scope: sec
type: non-functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

The server **shall** answer 401 to every request that carries neither the
correct token nor the cookie holding it.

## Rationale

The token is what separates the user from any other local process or web page.

## Acceptance criteria

1. `GET /api/graph` without a cookie answers 401.
2. `GET /?token=wrong` answers 401.
3. `GET /api/graph` with the cookie answers 200.

## Notes

In embed mode (`--embed`, scope `ext`) the token travels in a header or the
query string instead of a cookie.
