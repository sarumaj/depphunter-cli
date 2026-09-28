---
id: REQ-AUTH-002
title: netrc credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read the machine, login and password entries of the
netrc file the go command reads - `NETRC`; else on Windows `~/_netrc` when it
exists; else `~/.netrc` - and file each under its machine.

## Rationale

A netrc is what git and curl read, and therefore how a Go proxy, a pip mirror or
a private repository is most often reached.

## Acceptance criteria

1. A request to a netrc machine carries its Basic credential; a request to
   another host carries none.
2. With `NETRC` set, `~/.netrc` is not read.
