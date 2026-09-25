---
id: REQ-AUTH-010
uuid: 7751338c-f945-4916-a61a-2f09686549e7
title: An encrypted password is not sent
scope: auth
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M16
verification:
  - unit
---

## Statement

The system **shall not** send an encrypted password as a credential.

## Rationale

An encrypted password needs a key that is not available here; sent as ciphertext
it is a 401 either way.

## Acceptance criteria

1. A NuGet source with only an encrypted `Password` gets no credential.
2. A Maven server whose password is encrypted (`{...}`) gets no credential.

## Notes

NuGet reads only `ClearTextPassword`, so an encrypted NuGet password is left
alone. The Maven reader skips a password holding a `{...}`, wherever it sits in
the value, even for a server a mirror or repository names; the test now has a
mirror naming each encrypted server.
