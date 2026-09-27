---
id: REQ-HASKELL-006
uuid: f80a3d0f-695e-40db-9e06-27771e8c170f
title: Modules resolved to the files declaring them
scope: haskell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve an imported module to the project file whose
module header declares it: in the importer's own package (its component's
source directories first), then a generated module's Alex, Happy, c2hs or
hsc2hs source probed under the package's source directories by its path, then
a package of the same project (a `cabal.project` or `stack.yaml` directory, or
its `packages:`) or one the importer depends on. A file of no package takes the
nearest declaration anywhere. `Main` is dropped; `Paths_<pkg>`,
`PackageInfo_<pkg>` and `Build_<pkg>` resolve to that package's description.

## Rationale

Module headers say which file is which module whatever layout the
package description gives, including files of no package at all; restricting
the search to related packages keeps a test fixture's copy of a module from
capturing imports of the real one.

## Acceptance criteria

1. `shop-app/app/Main.hs`'s `import Shop.Cart` goes to
   `shop-core/src/Shop/Cart.hs`.
2. `import Shop.Parser` goes to `shop-core/src/Shop/Parser.y`;
   `docs/Tutorial.lhs` (no package) finds `Shop.Types`.
