---
id: REQ-OCAML-004
title: Modules resolved to project files
scope: ocaml
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A module path **shall** resolve, in order, to the module's file in the
importer's component - the dune library, executable or test building it
(its directory, the tree `(include_subdirs unqualified)` adds, a
subdirectory's `Sub.Module` under `qualified`, the modules its `(modules)`
field selects), or the file's directory when no stanza builds it - with the
implementation before the interface and before what generates it (`.mly`,
`.mll`, `t.cppo.ml`); to a module the opened modules bring into scope (the
file's `open`s and its component's `-open` flags: a module a local opened
file declares or includes, a module of an opened local library); to a
module of a local library the component uses (`Lib.Module` of a wrapped
library, its main module, any module of an unwrapped one); and, after the
standard library and the component's packages, to a local library or a
module file the repository has once. A library of the repository named in
`libraries` **shall** resolve to its dune file.

## Rationale

This is the scope dune gives a module name; opening a module of one's own
(`open Import`) that aliases others is a common idiom.

## Acceptance criteria

1. `bin/main.ml` resolves `Cart` (through `-open Shop`), `Shop.Cart` and
   `Shop.Price` to the library's files; `lib/cart.ml` resolves `Money` and
   `Json` to `lib/import.ml` and `Strings` to the unwrapped library's
   `core/util/strings.ml`.
2. `qual/api.ml` resolves `Net.Client` to `qual/net/client.ml`, and
   `qual/net/client.ml` its sibling `Server`; `lib/lexer.mll` resolves
   `Parser` to `lib/parser.mly`.
3. `libraries: shop` resolves to `lib/dune`.
