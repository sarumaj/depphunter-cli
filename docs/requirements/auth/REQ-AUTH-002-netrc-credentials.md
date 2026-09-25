---
id: REQ-AUTH-002
uuid: 2ddf4e2f-b30a-4b02-bae6-03bc9b55b2f7
title: netrc credentials
scope: auth
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M16
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The system **shall** read the machine, login and password entries of the user's
`~/.netrc` or `~/_netrc` and file each under its machine.

## Rationale

A netrc is what git and curl read, and therefore how a Go proxy, a pip mirror or
a private repository is most often reached.

## Acceptance criteria

1. A request to a netrc machine carries its Basic credential; a request to
   another host carries none.
