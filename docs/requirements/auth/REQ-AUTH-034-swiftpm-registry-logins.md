---
id: REQ-AUTH-034
title: SwiftPM registry logins from the netrc
scope: auth
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The credential `swift package-registry login` keeps in the netrc **shall** be
sent to each registry the user's `registries.json` names as SwiftPM sends it:
the netrc password as a Bearer token, below the registry's path only, when the
file's `authentication` entry for the registry's host (with its port) says
`token`, or when it has no entry and the netrc login is `token`; otherwise the
netrc pair as Basic, as any netrc credential goes. A repository's
`registries.json` **shall not** decide how a credential is sent.

## Rationale

A registry that takes a token refuses the token sent as a Basic pair; the
netrc alone cannot say which form a registry wants.

## Acceptance criteria

1. A `token` registry at a path gets the Bearer token there, and the host's
   other paths the netrc pair; a `basic` registry the pair; a registry with no
   entry and the login `token` the token; a registry the netrc has nothing for
   nothing.

## Notes

What SwiftPM keeps in the macOS keychain (its default there, unless
`--disable-keychain`) cannot be read without the keychain's consent, and is
not.
