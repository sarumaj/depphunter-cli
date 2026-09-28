---
id: REQ-AUTH-007
title: Credential helper constraints
scope: auth
type: constraint
priority: must
status: implemented
verification:
  - unit
  - inspection
---

## Statement

A credential helper name **shall** be a bare name of letters, digits, `_` and
`-`; the helper **shall** be resolved on `PATH` only and bounded by a timeout of
10 seconds; an identity token (user name `<token>`) **shall** not be taken as
a password, only as the registry's identity token
([REQ-AUTH-030](REQ-AUTH-030-registry-identity-tokens.md)).

## Rationale

The helper is the only program executed that was not named on the command line;
a configuration file may contribute a name and nothing more. Only a registry's
token endpoint accepts an identity token.

## Acceptance criteria

1. A helper named `../../evil` or by an absolute path is not run.
2. A helper answering with the user name `<token>` contributes no Basic
   credential.
