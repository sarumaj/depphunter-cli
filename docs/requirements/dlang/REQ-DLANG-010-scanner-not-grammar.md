---
id: REQ-DLANG-010
title: Read by a scanner, not the grammar
scope: dlang
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

D **shall** be read by a lexer of its own, not a tree-sitter grammar: nested
`/+ +/` comments, every string form and token string as one token, char
literals, numbers with `..` ranges, `#line` and a leading `#!` skipped,
`__EOF__` ending the file; and a block scanner over the tokens (aggregate,
conditional and attribute, body and expression braces). Any input
**shall** be read in time linear in its size without a panic.

## Rationale

The vendored grammar took 27 ms (dub) to 71 ms (vibe.d, mir-algorithm)
per file, up to 1.1 s for one file, and parsed 27 of 563 files with errors;
imports and declarations are token-level.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   extract without a panic within the time bound.
