---
id: REQ-PURESCRIPT-008
uuid: 1a11e3fc-3bc3-4708-9766-3c430e6a3717
title: Dependencies from spago.lock
scope: purescript
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The PureScript plugin **shall** answer what a package depends on from the
repository's records: a `spago.lock` entry's `dependencies`, each pinned by
that lock, else the `dependencies` an extra package lists (`spago.yaml`
`extraPackages`, a `packages.dhall` override), each as its workspace
decides it.

## Rationale

`spago.lock` records the whole build plan with each package's dependencies,
so `--resolve-depth` needs no network.

## Acceptance criteria

1. aff 7.1.0 depends on effect 4.0.0 and prelude 6.0.1; the forked override
   depends on prelude and maybe at the package set's version.
