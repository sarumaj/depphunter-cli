---
id: REQ-AUTH-007
uuid: a7c57b13-97e5-4245-a9a8-b34362efa86a
title: Credential helper constraints
scope: auth
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M16
verification:
  - unit
  - inspection
---

## Statement

A credential helper name **shall** be a bare name of letters, digits, `_` and
`-`; the helper **shall** be resolved on `PATH` only and bounded by a timeout of
10 seconds; an identity token (user name `<token>`) **shall** be declined.

## Rationale

The helper is the only program executed that was not named on the command line;
a configuration file may contribute a name and nothing more. Only a registry
accepts an identity token.

## Acceptance criteria

1. A helper named `../../evil` or by an absolute path is not run.
2. A helper answering with the user name `<token>` contributes no credential.
