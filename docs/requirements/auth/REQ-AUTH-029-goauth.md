---
id: REQ-AUTH-029
title: GOAUTH decides whether a Go proxy gets the netrc credential
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

A request for a Go module's `go.mod` from a module proxy **shall** carry the
netrc's credential for the proxy host
([REQ-AUTH-002](REQ-AUTH-002-netrc-credentials.md)) only when the go command
would send it: `GOAUTH` (from the environment, else the go env file) unset, or
a `;`-separated list with a `netrc` entry and no `off` entry. `GOAUTH=off`, or
a list of only `git <dir>` and command entries, **shall** leave the netrc's
credential out of Go proxy requests. The `git <dir>` and command forms
**shall not** be executed. A credential another file holds for the same host,
filed over the netrc's, and every request of another ecosystem **shall** be
unaffected.

## Rationale

Go 1.24 made the netrc one of several `GOAUTH` providers: with `GOAUTH=off`
the go command sends nothing, and a proxy that answers the netrc credential
would give depphunter an answer the go command itself never gets. Running the
configured `git credential` or command would execute a program a variable
names, which depphunter does only for Docker credential helpers
([REQ-AUTH-007](REQ-AUTH-007-credential-helper-constraints.md)).

## Acceptance criteria

1. Against a stub proxy answering 401 without credentials: `GOAUTH` unset,
   `netrc` or `git /src;netrc` answers; `off`, `git /src` or a command does
   not, and the request has no `Authorization` header.
2. `GOAUTH=off` in the file `GOENV` names is honored.
3. Under `GOAUTH=off` a non-Go request to the same host still carries the
   netrc credential, and a Go request carries a container-file credential
   filed over the netrc's.

## Notes

Credentials from `git credential fill` (`GOAUTH=git <dir>`) or from a command
are not available to depphunter; configure the same host in the netrc (with
`GOAUTH` listing `netrc`) for its questions to be answered. Modules the go
command fetches directly from version control (`GONOPROXY`, `GOPRIVATE`, an
entry `direct`) are not asked
([REQ-SUP-021](../sup/REQ-SUP-021-go-proxy-go-mod.md)).
