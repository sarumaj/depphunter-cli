---
id: REQ-SEC-002
uuid: cf2aedc6-f2ad-423a-9acd-cb9b2b595413
title: Per-run random token
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

The server **shall** generate a new random token of 24 bytes from a
cryptographic source on every run, and the URL it prints and opens **shall**
carry it as the `token` query parameter.

## Rationale

Other local users and web pages cannot guess the token, so they cannot read the
map even though they can reach the port.

## Acceptance criteria

1. Two runs print different tokens.
2. The printed URL ends with `/?token=` followed by 48 hexadecimal digits.
