---
id: REQ-OCAML-011
uuid: 967d505b-2ec7-4dd0-9c40-bf4bb97599c8
title: Read by a scanner, not the grammar
scope: ocaml
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

OCaml sources, dune's S-expressions and opam files **shall** be read by
scanners of their own, not a tree-sitter grammar: nested comments with the
strings inside them, strings, quoted strings (`{|...|}`, `{id|...|id}`,
`{%ext|...|}`), character literals told from type variables, polymorphic
variants, extension keywords (`let%lwt`) and binding operators (`let*`),
attributes, cppo and toplevel directives, and Menhir's `/* */` and `//`
comments. The scanners **shall** return a result for any input.

## Rationale

The vendored OCaml grammar was measured on yojson, base, irmin and
ocaml-lsp: 3.7 to 7 ms per file (1.28 s for one generated file) and ERROR
nodes in 2 to 11% of files (base: 28 of 291). The scanner and resolver
read dune's 5009 files in about 0.4 s. A panic in Extract would end the
analysis, so they were fuzzed.

## Acceptance criteria

1. Every prefix of every fixture file, and inputs cut inside each
   construct, extract without a panic; 100000 nested brackets do not
   exhaust the stack.
2. Paths after each trap (a comment holding `"*)"`, `'"'`, a quoted
   string, cppo lines) are read.
