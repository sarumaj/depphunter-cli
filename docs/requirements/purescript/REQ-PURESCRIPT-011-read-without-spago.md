---
id: REQ-PURESCRIPT-011
title: Read without running spago
scope: purescript
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

PureScript **shall** be read without running spago or purs and without
downloading packages: a module of a package that is not installed, not in
the curated table and not spelled by a listed package's name is dropped;
the versions a package set holds are not known offline; Dhall beyond
records, lists and merges (functions, `env:` imports, interpolated text) is
not evaluated; a module's name from an installed package is read from its
path under `src/`.

## Rationale

Which package provides a module is recorded only in that package; the
package set is a remote Dhall file.

## Acceptance criteria

1. `Mystery.Thing` is dropped.
