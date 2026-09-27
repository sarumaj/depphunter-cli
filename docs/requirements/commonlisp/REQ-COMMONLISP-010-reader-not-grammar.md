---
id: REQ-COMMONLISP-010
title: Read by a reader, not the grammar
scope: commonlisp
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Lisp sources, `.asd` files and `qlfile.lock` **shall** be read by a
reader of their own, not a tree-sitter grammar: `;` and nested `#| |#`
comments, strings, characters (`#\(`, `#\Space`), symbols with `|` and
`\` escapes and package prefixes (`pkg:sym`, `pkg::sym`), keywords and
`#:` symbols, quote, quasiquote, unquote, `#'`, vectors, structures,
pathnames, `#.` read-time evaluation (kept, marked), labels, bit vectors
and radix numbers. Reader conditionals **shall** read both branches:
`#+feature` and `#-feature` drop their feature expression and keep the
next form, since which features hold is known only to the implementation;
an expression false or true everywhere (`#+nil`, `#+(or)`, `#-(and)`)
drops it. Any input **shall** be read in time linear in its size without
a panic.

## Rationale

The vendored Common Lisp grammar took 30 ms per file on cl-ppcre, ironclad,
dexador and rove (835 ms for one file) and parsed 36 of their 191 files
with errors; the reader extracts lem's 755 files in about 0.2 s.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   are read without a panic within the time bound.
2. The reader test's datums read back as written.
