---
id: REQ-OCAML-006
title: dune files read
scope: ocaml
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read dune's S-expressions (comments `;`, `#| |#` and
`#;` datum comments, strings with escapes) and from `library`,
`executable(s)` and `test(s)` stanzas read the name (or the public name
alone), public names, `libraries` (including `re_export` and the libraries
a `select` chooses between), the ppx rewriters of `preprocess` and `lint`
(`pps`, `staged_pps`, `per_module`, up to `--`), `modules` (`:standard`
with `\` exceptions), `wrapped false`, `-open` in `flags`, and
`include_subdirs`. Each library and rewriter **shall** be an import
(`libraries: x`, `pps: x`).

## Rationale

dune's `libraries` is the dependency declaration of OCaml code; rewriters
are dependencies too.

## Acceptance criteria

1. `lib/dune` imports lwt, lwt.unix, yojson, fmt.tty, logs.fmt, the local
   shop_core and qual, unix from its select and ppx_deriving.show.
2. A dune file with a datum-commented field, an escaped string, a list
   across lines and an unclosed stanza parses.
