---
id: REQ-HASKELL-009
uuid: 7cceb1e7-03a6-4899-bb83-0970c946e954
title: Packages found for modules
scope: haskell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve a module no project file declares to: GHC's own
packages (base, ghc-prim, ghc, template-haskell, integer-gmp, ghc-bignum,
ghc-boot, ghc-boot-th, ghc-heap, ghc-internal, rts) as the hidden
`haskell-std` island whether declared or not; else a Hackage package found, in
order, by a curated module table's entry for exactly that module, the table's
longest matching prefix among the declared packages, a declared package named
by a run of the module's segments (`Network.HTTP.Client`: `http-client`), the
table's longest prefix at all, a declared package named by its first word
(`Data.Hermes`: `hermes-json`), a package the project's plan, freeze file or
lock names by a run of segments, and else the module's first segment that does
not name a subject (Data, Control, Network, Text, ...), in lower case.

## Rationale

Haskell modules do not name their package; the boot packages other than GHC's
own library (containers, text, mtl, ...) are released on Hackage and declared
like any other, so they are Hackage packages.

## Acceptance criteria

1. `Data.Map.Strict` goes to `containers`, `Control.Monad.State` to `mtl`,
   `Data.List` and `GHC.Generics` to `base` in `haskell-std`.
2. `Data.Unknown.Thing`, declared nowhere, is the unresolved package
   `unknown`; `System.Win32.Console` the unresolved `Win32`.
