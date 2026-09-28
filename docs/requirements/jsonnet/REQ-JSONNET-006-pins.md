---
id: REQ-JSONNET-006
title: Pins
scope: jsonnet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A git dependency **shall** be pinned by the lock's version, with the
declared version requested when it differs. Without a lock entry a commit
(40 or 64 hexadecimal digits) **shall** pin, a tag (`v1.2.3`, `2.0`)
**shall** be shown neither pinned nor floating, and a branch (`main`,
`release-0.14`) or no version **shall** float. A repository on a server
other than GitHub, GitLab, Bitbucket, Codeberg or sourcehut **shall** be
the package's origin.

## Rationale

The repository's rule for git references: commits pin, tags are
movable in principle, branches move.

## Acceptance criteria

1. ksonnet-util is pinned by the lock with `master` requested, node-mixin
   shows `v1.8.0` unpinned and not floating, `main` floats, and the
   private repository is pinned by commit with its remote as origin.
