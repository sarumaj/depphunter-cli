---
id: REQ-PURESCRIPT-007
uuid: f3ee8132-f57d-46c8-bd6e-1fd84404b5b9
title: Modules resolved to packages
scope: purescript
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A module no project file declares **shall** resolve, in order: a Prim module
(`Prim`, `Prim.Row`, `Prim.RowList`, `Prim.Symbol`, `Prim.TypeError`,
`Prim.Boolean`, `Prim.Coerce`, `Prim.Int`, `Prim.Ordering`) to the
compiler's built-ins; to the package spago installed that provides it
(`.spago/p/<name>-<version>/src` and `.spago/p/<name>/<ref>/src` of spago
0.93 and later, `.spago/<name>/<version>/src` of spago 0.20,
`bower_components/purescript-<name>/src`, in the nearest directory at or
above the project that has them); to the listed package that a curated
module table (prelude's modules, `Data.Map` ordered-collections, `Effect.Aff`
aff, `Control.Monad.State` transformers, ...) or the package's own name
(folded segments, optionally after `Data`, `Control`, `Effect`, `Test` or
`Type`: `Node.FS` node-fs, `Data.Maybe` maybe) names, the longer match
winning and the table on a tie; to a file another project declares it in;
to the unresolved package the table names; else it is dropped.

## Rationale

A module's name does not say its package; the installed package says for
certain, and the table and the listed packages cover what is not installed.

## Acceptance criteria

1. `Prim.Row` is a built-in; `Glitter.Sparkle` and `Registry.PackageName`
   are the packages installed in `.spago/p/`; `Data.Map`, `Node.FS.Aff`,
   `Test.Spec` and `Control.Monad.State` go to their packages;
   `Data.Argonaut.Core` is the unresolved argonaut-core.
