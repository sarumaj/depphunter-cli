---
id: REQ-OCAML-007
title: opam manifests read
scope: ocaml
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read opam's format (`#` and `(* *)` comments,
strings, `"""` strings, sections) for `depends`, `depopts` and
`pin-depends`, and dune-project's `(package (name) (depends) (depopts))`
and dune package management's `(pin (url) (package (name)))`. A
dependency's filter gives its version constraint as written, with `&`/`|`
(`>= 5.6 & < 6`), without variables (`with-test`, `build`, `os != "win32"`);
`{= "1.2"}` is exact. Every dependency and pin **shall** be an import of
its package (`depends: x`, `depopts: x`, `pin-depends: x`, `pin: x`); a
lock's are its pinned versions. A package the repository describes (a
`<name>.opam`, a dune-project package) **shall** resolve to its
description.

## Rationale

opam files and dune-project are how an OCaml project declares its packages;
dune generates the former from the latter.

## Acceptance criteria

1. dune-project imports lwt, yojson, alcotest, ppx_deriving, fmt, logs,
   conf-libev, cmdliner and re, and `shop` resolves to `shop.opam`; its
   datum-commented dependency is not read.
2. An opam file's alternatives `("ssl" | "tls" {>= "0.17" | = "0.16"})`
   give both packages, `{= version}` is kept as a constraint and a filter
   comparing a variable is not one.
