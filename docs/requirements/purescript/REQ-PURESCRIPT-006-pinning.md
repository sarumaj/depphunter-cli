---
id: REQ-PURESCRIPT-006
title: Pinning
scope: purescript
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A package `spago.lock` records **shall** be pinned at its version (with the
range `spago.yaml` asks for as the requested version), and one installed
from git **shall** be named by its repository (and subdirectory), pinned by a
commit, its origin being the repository. A registry version in
`extraPackages` pins; a git tag (`ref` in `spago.yaml`, `version` of a
`packages.dhall` override) is shown and neither pinned nor floating; a
range (`>=7.0.0 <8.0.0`, bower's `^6.0.0`) floats; a bare name decided by a
package set is neither pinned nor floating, with the package set's name
(`registry 60.0.0`, `psc-0.15.0-20220507`) as its version; a bare name
without a package set floats. A package that no manifest of the importing
file's project lists is unresolved.

## Rationale

A package set fixes one version of every package it holds, but which one is
not known without downloading the set.

## Acceptance criteria

1. aff is pinned at 7.1.0, requested as `>=7.0.0 <8.0.0`; console's version
   is `registry 60.0.0`; dom-indexed is
   `github.com/purescript-halogen/purescript-dom-indexed` at `v11.0.0`,
   neither pinned nor floating; forked is pinned by its commit.
