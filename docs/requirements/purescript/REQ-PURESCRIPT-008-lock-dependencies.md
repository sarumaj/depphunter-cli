---
id: REQ-PURESCRIPT-008
title: Dependencies from spago.lock and .spago
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
decides it, else the dependencies (not the test dependencies) the manifest
of the package spago installed into the workspace's `.spago/` lists
(`spago.yaml`, else `spago.dhall`, else `purs.json`; `p/<name>-<version>/`
and `p/<name>/<ref>/` of spago 0.93, `<name>/<version>/` of spago 0.20),
reported as installed. A dependency the lock or extra packages do not
decide and spago installed too is pinned to what spago installed.

## Rationale

`spago.lock` records the whole build plan with each package's dependencies,
so `--resolve-depth` needs no network; without it, each installed package
carries its own manifest.

## Acceptance criteria

1. aff 7.1.0 depends on effect 4.0.0 and prelude 6.0.1; the forked override
   depends on prelude and maybe at the package set's version.
2. Without a lock, the installed halogen 7.0.0 depends on aff 7.1.0 and
   prelude 6.0.1 as installed and on dom-indexed at the package set's
   version, not on its test dependency spec; a git package's `spago.yaml`
   under its ref and a spago 0.20 package's `spago.dhall` answer too; a
   garbage `spago.yaml` gives none.
