---
id: REQ-NIM-010
title: Read by a scanner, not the grammar
scope: nim
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Nim **shall** be read by a lexer and an indentation-based scanner of its
own, not a tree-sitter grammar: the lexer knows `#` and `##` comments,
nested `#[ ]#` and `##[ ]##` block comments, strings with escapes, raw and
generalized raw strings, triple-quoted strings, character literals,
number suffixes (`1'i32`) and backquoted names; the scanner splits
statements at the line starts outside brackets and tracks top level, `when`
branches, type and constant sections and bodies by indentation. Any input
**shall** be read in time linear in its size without a panic, and so
**shall** the manifests.

## Rationale

The vendored Nim grammar is licensed under the MPL-2.0 and stubbed out of
this build; it is neither used nor measured. The plugin needs only the top
level.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct are
   read without a panic within the time bound.
