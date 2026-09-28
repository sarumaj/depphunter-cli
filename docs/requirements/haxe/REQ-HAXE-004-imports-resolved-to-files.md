---
id: REQ-HAXE-004
title: Imports resolved to files
scope: haxe
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** index every module of the repository by the package it
declares (`package a.b;` in `C.hx` is module `a.b.C`; a file without a package
declaration is a module of the top-level package), so that imports resolve
whatever the class paths. An import or a qualified name **shall** resolve to
the file of its longest leading module path (`a.b.C.D` is the type `D` in
`a/b/C.hx`, `a.b.C.field` a field of it), the file nearest the importing one
when several class paths declare the module; `import a.b.*` of a package of
the repository **shall** expand into one import per module of the package (at
most 100, the importing file excluded) and `import a.b.C.*` resolve to `C`'s
file. A module importing itself **shall** be dropped. Every module **shall**
have an edge to each `import.hx` whose imports and usings the compiler applies
to it: one in the module's directory or a directory above it up to its class
path (the directory its package starts in), shown as `import.hx (<path>)`; a
module whose directory does not spell its package takes only its own
directory's, and `import.hx` itself none.

## Rationale

Class paths come from `-cp`, `haxelib.json`, `Project.xml`, lix and
editor settings alike; the package a module declares is what the compiler
checks, so it finds the file without knowing the class path.

## Acceptance criteria

1. `shop.model.Cart.CartItem` resolves to `src/shop/model/Cart.hx`,
   `shop.util.*` to `src/shop/util/Money.hx` and
   `src/shop/util/Strings.hx`, `shop.util.Money.*` to
   `src/shop/util/Money.hx` and `Config` to `shared/Config.hx`.
2. `mylib.Thing` in `game/src/Game.hx` resolves to
   `game/libs/mylib/src/mylib/Thing.hx`, a library in development in the
   repository.
3. `src/a/b/C.hx` (package `a.b`) has edges to `src/a/b/import.hx`,
   `src/a/import.hx` and `src/import.hx`, not to the `import.hx` above
   `src/`; `odd/D.hx` (package `x.y`) only to `odd/import.hx`.
