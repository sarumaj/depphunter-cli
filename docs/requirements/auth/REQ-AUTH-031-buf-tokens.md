---
id: REQ-AUTH-031
title: Buf Schema Registry tokens
scope: auth
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The token sent to a Buf Schema Registry host **shall** be the one buf sends
there: `BUF_TOKEN`'s `<token>@<host>` entry for it (comma-separated entries),
else the password of the netrc's machine entry for the host, where
`buf registry login` keeps it, sent as a Bearer token. A `BUF_TOKEN` without a
host **shall** be sent to buf.build alone. The token **shall** be sent only
over https or to this machine.

## Rationale

Private modules answer only to a token; buf would send a bare token to any
registry, but here the registry is named by the repository's module names.

## Acceptance criteria

1. `a-token@buf.build, b-token@BUF.Other.example` gives each host its token; a
   netrc entry gives buf.corp.example its password; a bare token reaches
   buf.build only.
