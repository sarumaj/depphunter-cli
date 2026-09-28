---
id: REQ-SUP-021
title: Go dependencies from the module proxy
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a Go module's requirements from the `go.mod` the
module proxy serves for its version, without fetching an archive, and **shall**
take only its direct (not `// indirect`) requirements.

## Rationale

A module proxy serves a module's own go.mod on its own.

## Acceptance criteria

1. Against a stub proxy the client requests `<module>/@v/<version>.mod` and
   returns its requirements with their versions, pinned.

## Notes

The proxies of a `GOPROXY` list are asked in turn as the go command does
([REQ-SUP-063](REQ-SUP-063-additive-sources-fall-back-to-the-public-index.md)).
Whether a proxy is sent the netrc's credential is `GOAUTH`'s to say
([REQ-AUTH-029](../auth/REQ-AUTH-029-goauth.md)).

A module the go command fetches directly from version control - one matching
`GONOPROXY` or `GOPRIVATE`, or reached through a `direct` entry - is not asked
(the report says the package is private, or that the proxy list ends). Reading
its `go.mod` at the version from the forge's raw-file URL instead
(`raw.githubusercontent.com/<owner>/<repo>/<version>/go.mod`, GitLab's
`/-/raw/`) was considered and not done: the module is private by the user's
own configuration, and the request would name it, and its version, to a host
outside that configuration - the disclosure `GOPRIVATE` exists to prevent.
Cloning with the machine's git credentials would execute git against a URL
the module path chooses.
