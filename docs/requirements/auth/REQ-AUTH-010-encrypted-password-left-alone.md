---
id: REQ-AUTH-010
uuid: 7751338c-f945-4916-a61a-2f09686549e7
title: An encrypted password is not sent
scope: auth
type: constraint
priority: must
status: partial
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

Partial: NuGet reads only `ClearTextPassword`, so an encrypted NuGet password is
left alone. The Maven reader does not recognise an encrypted `{...}` password: a
server whose id a mirror or repository names is sent with the ciphertext as its
password. The existing test uses an encrypted server that no mirror names, so it
passes for the wrong reason.
