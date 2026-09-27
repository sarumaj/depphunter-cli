---
id: REQ-SUP-061
uuid: 339d1dcc-046c-4024-a934-8c1bb47fa67a
title: The Alire community index for crate dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a crate of the
`alire` island depends on from the Alire community index, a git
repository of release manifests read as files:
`<index>/index/<first two letters>/<crate>/<crate>-<version>.toml`. Only
an exact version (a lock file's, an `=1.2.3` constraint's) **shall** be
asked about, since a file server cannot list a crate's releases; a range,
a branch or a commit is reported as having no version to ask for. The
release's `depends-on` crates (every `case(...)` alternative counted)
**shall** be returned with their constraints, an exact one as its version.
The `stable-1.4.0` branch of alire-project/alire-index on
raw.githubusercontent.com is the public index; a name that is not an Alire
crate name is not asked.

## Rationale

The lock file's solution is complete only for the crates the project
locked; the index describes every release.

## Acceptance criteria

1. aws 25.2.0 asks `index/aw/aws/aws-25.2.0.toml` only and yields
   gnatcoll 25.0.0 (pinned), make, winsock and xmlada with their
   constraints; `^25.0`, a commit, a branch and no version ask nothing.
