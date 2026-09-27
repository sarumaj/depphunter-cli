---
id: REQ-JULIA-011
title: Read by a scanner, not the grammar
scope: julia
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Julia sources **shall** be read by a lexer and reader of their own, not a
tree-sitter grammar: nested `#= =#` comments, strings and triple-quoted
strings with `$(...)` interpolation holding code (and strings), string
macros (`raw""`, `r""`, `md"""..."""`) without interpolation, command
literals, character literals told from the adjoint `'`, quoted symbols
(`:end`, `:module`), identifiers with `!` and Unicode, `end` and
`begin` as indices inside brackets, and `for`/`if` clauses of
comprehensions and generators. The reader **shall** return a result for any
input, in time linear in its size.

## Rationale

The vendored Julia grammar was measured on DataFrames, Flux and Documenter:
314, 21 and 30 ms per file (7.9 s for one DataFrames test file) and ERROR
nodes in 21 of 74, 10 of 101 and 6 of 102 files. The scanner reads
OrdinaryDiffEq's 1156 files in about 0.3 s with every block balanced. A
panic in Extract would end the analysis, so it was fuzzed.

## Acceptance criteria

1. Every prefix of every fixture file and 1 MB runs of brackets,
   interpolations, `begin`, `end`, comments, quotes and definitions
   extract without a panic.
2. Imports and definitions after each trap are read, and none inside
   strings or comments.
