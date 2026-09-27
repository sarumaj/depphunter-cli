---
id: REQ-OCAML-002
uuid: 91a21701-1305-45cd-95ed-1624ef95eeb5
title: Module references read
scope: ocaml
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

As OCaml has no import statement, the plugin **shall** read the module
paths a source names as its imports, one per path of at most two segments:
a capitalized name followed by a dot (`Foo.bar`, `Foo.Bar.t`, `Foo.(e)`,
`r.Foo.field`), and a module where only a module can stand - after `open`,
`open!`, `let open`, `include`, `module M =` (functor applications and their
arguments too), `module type of` and `(module M)`. dune's `Lib__Module`
names are read as `Lib.Module`. A capitalized name not followed by a dot
(a constructor, an exception) **shall not** be an import, and neither shall
a module the file binds itself (`module M = struct`, a functor's parameter,
`let module`, `(module M : S)`, `module rec ... and M`). A toplevel
script's `#require "a,b"` **shall** import each library. Specs are the
path, prefixed with `open` or `include` for those.

## Rationale

A file depends on another module exactly where it names it; dune computes
its build order the same way (ocamldep).

## Acceptance criteria

1. `lib/cart.ml` imports `open Import`, `open Lwt.Syntax`, `Lwt`, `Price`,
   `List`, `Strings`, `Logs`, `Fmt` and `Lwt_unix`, and neither `Some`,
   `Ok`, `Empty` nor its own `Id` and `Local`.
2. In a sample, paths inside comments, strings, quoted strings,
   character literals and polymorphic variants are not read; `Real.Make
   (Real.Arg)`, `(module Real.Packed : Real.S)`, `r.Real.Field.x` and
   `Real__Wrapped.x` are.
3. `scripts/setup.ml`'s `#require "yojson,lwt.unix"` imports yojson and
   lwt.unix.
