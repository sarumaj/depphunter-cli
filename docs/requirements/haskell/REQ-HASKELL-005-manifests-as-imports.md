---
id: REQ-HASKELL-005
title: Package descriptions and projects as imports
scope: haskell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** make each `build-depends` and `setup-depends` entry of a
`.cabal` file and each `dependencies` entry of `package.yaml` an import of the
package it names (a package's use of its own library is dropped; another
package of the repository resolves to its description), each `packages:` entry
of `cabal.project` and `stack.yaml` an import of that package's description
(globs are not expanded), each `source-repository-package` of `cabal.project`
and each `extra-deps` entry of `stack.yaml` an import of the package it
builds.

## Rationale

Executables and test suites use packages no library module imports, and a
project's pins are only visible on the map when the project file points at
them.

## Acceptance criteria

1. `cabal.project`'s `packages: shop-app/` goes to
   `shop-app/shop-app.cabal`; `shop-app.cabal`'s `build-depends: shop-core` to
   `shop-core/shop-core.cabal`.
2. `stack.yaml`'s `extra-deps: left-pad-hs` goes to the `left-pad` package
   that `stack.yaml.lock` says the repository holds.
