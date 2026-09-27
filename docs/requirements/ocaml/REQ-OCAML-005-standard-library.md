---
id: REQ-OCAML-005
uuid: e4197c68-faf2-4ef0-b980-9058ecad25fa
title: Standard library
scope: ocaml
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Modules of OCaml's standard library (`List`, `Printf`, `Hashtbl`, ... as
package `stdlib`) and of the libraries the compiler ships (`Unix` as
`unix`, `Str` as `str`, `Thread` as `threads`, `Dynlink`, compiler-libs'
modules for a component using compiler-libs) **shall** resolve to the
hidden `ocaml-std` island, as **shall** those libraries named in
`libraries`. After a file opens a package that replaces the standard
library (Base, Core, Batteries, Containers), a standard library module name
**shall** be that package's. The compiler and its virtual packages
(`ocaml`, `ocaml-base-compiler`, `base-unix`, ...) are not imports of a
manifest.

## Rationale

The standard library comes with the compiler; `open Core` makes `List`
Core's.

## Acceptance criteria

1. `List`, `Printf`, `Lexing` and `Sys` are `stdlib`, `Unix` and
   `libraries: unix` are `unix`, `Str` is `str`.
2. `core/util/strings.ml`'s `String` after `open Base` is Base's.
3. `shop.opam`'s `ocaml` and the lock's `ocaml-base-compiler` are not
   imports.
