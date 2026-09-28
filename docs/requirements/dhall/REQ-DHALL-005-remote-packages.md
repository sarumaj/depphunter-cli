---
id: REQ-DHALL-005
title: Remote packages
scope: dhall
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A URL import **shall** be a package of the "Dhall packages" island: the
Prelude (`github.com/dhall-lang/dhall-lang/Prelude`) when it comes from
prelude.dhall-lang.org or from the Prelude directory of dhall-lang's
repository; the repository (`github.com/org/repo`, `gitlab.com/group/project`)
when GitHub, GitLab or jsDelivr serve its raw files, with the reference as
version; otherwise the host and the path up to the first segment that is a
version (the version), or the URL's directory. A `sha256:` hash **shall**
pin the import; without one, a URL naming no version or a branch
**shall** float, and a version or commit **shall** be shown, neither pinned
nor floating.

## Rationale

Dhall refuses content that does not match an import's hash, which freezes
it as a lock would (like CMake's `URL_HASH`); a version in a URL is only a
convention of the server.

## Acceptance criteria

1. The Prelude with a hash is pinned at `v23.0.0`; dhall-kubernetes from
   `master` floats; the Prelude from dhall-lang's repository at `v17.0.0` is
   shown unpinned; an `example.com/dhall/v1.2.0/...` import with a hash is
   pinned as `example.com/dhall`; jsDelivr, GitLab, a port, a user and a
   query are named as described.
