---
id: REQ-SUP-070
title: A Racket package catalog for package dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a package of the
`raco` island depends on from a package catalog, as raco consults one:
`<catalog>/pkg/<name>` answers a `read`-able hash table whose `dependencies`
have the shape of an info.rkt's `deps` (REQ-RACKET-012). The dependencies
**shall** be returned as the racket plugin names them: `base` and `racket` are
the `racket-std` island, a `#:version` is a minimum (never pinned), a source
URL names the package raco derives from it. An answer that is not a hash table
**shall** be taken as the catalog not having the package. A name that is not a
Racket package name is not asked.

pkgs.racket-lang.org is the public index.

## Rationale

raco keeps no lock file and installs a package's dependencies from what the
catalog lists for it.

## Acceptance criteria

1. An entry listing `"base"`, `("rackunit-lib" #:version "1.2")` and a git URL
   yields racket-std base, rackunit-lib at 1.2 (floating) and the package the
   URL names.
2. An entry whose dependencies are only in its `versions` table's `default`
   yields those; `#f` is "not found".

## Notes

A catalog records a package's current source only, so the answer is the same
whatever version was asked, and the version-specific entries of a catalog are
not selected by Racket version. The catalogs a Racket installation is
configured with (`raco pkg config catalogs`, kept in the installation's
`etc/config.rktd`) are not read: where Racket is installed is not known, so
only the public catalog is asked. The sandbox proxy blocks
pkgs.racket-lang.org; the protocol was taken from Racket's catalog-protocol
documentation and raco's client source, and verified with a stub server only.
