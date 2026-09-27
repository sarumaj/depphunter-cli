---
id: REQ-PURESCRIPT-010
uuid: d5ee562a-467d-48e7-9bbd-b2d99f7d6ac6
title: Read by a scanner, not the grammar
scope: purescript
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

PureScript **shall** be read by a lexer of its own, not a tree-sitter
grammar: `--` and (non-nesting) `{- -}` comments, strings with escapes and
gaps, `"""` raw strings, characters, identifiers with primes, qualified
names as one token, `∷`, operators and brackets. The module header runs to
its `where`; a declaration starts at a token first on its line in the
column of the first declaration or further left. Any input **shall** be read
in time linear in its size without a panic, and so **shall** the Dhall
reader's.

## Rationale

The vendored grammar took 86 ms per file and parsed 87 of 380 files of
purescript-prelude, purescript-halogen, purescript-halogen-realworld and
spago with errors; everything needed is token-level.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   extract without a panic within the time bound.
