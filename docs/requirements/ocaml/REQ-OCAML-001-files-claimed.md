---
id: REQ-OCAML-001
uuid: 0f22b860-2c71-4505-a3f5-7ec100c1c9f9
title: Files claimed
scope: ocaml
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The ocaml plugin **shall** claim OCaml implementations (`.ml`), interfaces
(`.mli`), ocamllex lexers (`.mll`) and Menhir or ocamlyacc grammars
(`.mly`), dune's `dune`, `dune-project` and `dune-workspace` files, and
opam's package descriptions (`*.opam`, `opam`) and locks (`*.opam.locked`,
`opam.locked`), except what lies under dune's `_build`, opam's local switch
`_opam` or esy's `_esy`. dune files, dune-project, opam files and locks
**shall** have cache classes of their own, since they share an empty or
unusual extension.

## Rationale

dune describes what OCaml sources form which library; opam and
dune-project say which packages the libraries come from. `_build` holds
copies of the sources, `_opam` the installed packages.

## Acceptance criteria

1. The fixture's sources, dune files, dune-project, dune-workspace, opam
   files and lock are analyzed; `_build/default/lib/cart.ml` and
   `_opam/lib/lwt/lwt.mli` are not.
2. `dune`, `dune-project`, `dune-workspace`, `opam`, `x.opam.locked` and
   `a.ml` have six different cache classes; `shop.opam.template` and
   `dune.lock/lwt.pkg` are not claimed.
