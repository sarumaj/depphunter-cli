---
id: REQ-RACKET-006
uuid: ce657b3d-4b7c-4b2e-b7dc-5ece2166d8ba
title: info.rkt dependencies and pinning
scope: racket
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An `info.rkt`'s `deps` and `build-deps` entries (quoted lists, `(list
...)` and `append` forms, `("name" #:version "1.2")` and bracketed entries)
**shall** be imports: a package of the repository by that name to its
`info.rkt`, a directory inside the repository to its `info.rkt`, `base` and
`racket` to the hidden `racket-std` island, and other packages to the
`raco` island named as raco names them (a git or archive source by the last
element of its path or of its `?path=`, without `.git`). No lock file
exists: a `#:checksum` or a git source's `#<commit>` **shall** pin; a
version tag in `#ref` **shall** be shown as neither pinned nor floating; a
branch, a `#:version` (a minimum) and no version **shall** float. A git
server other than the public forges, and a directory outside the
repository, **shall** be the package's origin.

## Rationale

raco installs the newest catalog version at or above `#:version`; only a
checksum or a commit names one version.

## Acceptance criteria

1. The fixture's `info.rkt` gives the targets its test lists: rebellion
   pinned by commit, guard floating on `main`, acme-log-lib at `v1.4.0` from
   git.acme.dev, typed-racket pinned by checksum, rackunit-lib floating
   from 1.10, widgets-lib to `libs/widgets-lib/info.rkt`.
2. Quasiquoted deps with an unquoted version and `(list ...)` entries are
   read.
