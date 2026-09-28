---
id: REQ-REGO-006
title: Read by a lexer, not the grammar
scope: rego
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Rego **shall** be read by a lexer of its own (comments, strings, raw
strings), not a tree-sitter grammar, and any input **shall** be read in
time linear in its size without a panic.

## Rationale

The vendored tree-sitter Rego grammar left errors in 282 of 431 files of
Regal and OPA's library (it predates `if` and `contains`), at 4.7 ms per
file; the lexer takes about 0.07 ms per file.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   are read without a panic within the time bound.
