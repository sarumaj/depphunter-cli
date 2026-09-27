---
id: REQ-OCAML-008
title: Packages resolved and pinned
scope: ocaml
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A library of `libraries` not in the repository **shall** be the opam
package named by its part before the first dot (`lwt.unix` is lwt,
`fmt.tty` fmt), with a small table for others (`findlib` is ocamlfind,
`zip` camlzip). A module path that is not the repository's or the standard
library's **shall** be the package of the component's library that provides
it (its main module by name, `Lwt_unix` of lwt by the package's prefix),
else of the one package whose module the file opens (`open Cmdliner`, then
`Term`), else of a package the manifests declare, else of a curated module
table (`Z` is zarith, `QCheck` qcheck-core) as unresolved; anything else is
dropped. The version **shall** come from the manifests governing the file
(its directory's and its ancestors', nearest first; all when none): a lock
(`*.opam.locked`, `dune.lock`) pins, with `Requested` the declared
constraint; a `pin-depends`/`pin` URL whose `#ref` is a commit pins, any
other ref floats, with the URL as origin; `{= x}` pins; a range floats as
written; no constraint floats. A package no manifest declares is
unresolved.

## Rationale

Libraries, not packages, are what dune and code name; opam's pins and locks
say which versions are in use.

## Acceptance criteria

1. `lib/dune`'s lwt is 5.7.0, pinned by `shop.opam.locked`, requested
   `>= 5.6 & < 6`; yojson is 2.1.0 pinned by `{= "2.1.0"}`; cmdliner is
   pinned to its pin-depends commit, re floats on `main`; alcotest floats.
2. `bin/main.ml`'s `Term` and `Cmd` after `open Cmdliner` are cmdliner;
   `test/test_shop.ml`'s undeclared `QCheck.Gen` is qcheck-core,
   unresolved; `core/dune`'s undeclared base is unresolved.
