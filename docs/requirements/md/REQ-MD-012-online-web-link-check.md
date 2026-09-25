---
id: REQ-MD-012
uuid: 07315723-3965-4294-ba4c-b8ff5a064fac
title: Web links checked only online, 404 and 410 only
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** check `http` and `https` links only under `--online`,
asking each distinct URL once with `HEAD` (repeated with `GET` when the host
answers 403, 405 or 501), at most 8 at a time, and **shall** report a
`link/gone` finding of low severity only when the host answers 404 or 410.

## Rationale

Only a host saying the page is gone is evidence of rot. A refusal, a rate limit,
a timeout and a server error are what hosts that block robots return, and
reading them as rot would report links that work in a browser.

## Acceptance criteria

1. Without `--online`, no external link is requested.
2. Of links answering 200, 404, 410, 403, 429, 500 and 405-then-200, exactly the
   404 and 410 ones are reported, titled `<url> answers <status>`.

## Notes

Requests carry a `User-Agent` naming depphunter and, where this machine holds a
credential for the host, that credential (scope `auth`).
