---
id: REQ-SEC-005
uuid: 97683a1f-fdc6-4c4f-8c42-4abcb9ea4477
title: Foreign Host header rejected
scope: sec
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §6
  - docs/REQUIREMENTS.md M1
verification:
  - integration
---

## Statement

When listening on a loopback address, the server **shall** answer 403 to every
request whose `Host` header is not `127.0.0.1`, `localhost` or `[::1]` with the
port it listens on.

## Rationale

A foreign `Host` header means a DNS-rebinding page is talking to the server.

## Acceptance criteria

1. A request with `Host: attacker.example:80` answers 403, even with the correct
   token.

## Notes

The check is off when the server is bound to a non-loopback address (`--addr`),
where no fixed set of host names applies.
