---
id: REQ-RACKET-010
uuid: 640a21c0-7eb7-44a3-a360-b92d562cb5ce
title: Read by a reader, not the grammar
scope: racket
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Racket, Scribble and `info.rkt` files **shall** be read by a reader of
their own, not a tree-sitter grammar: s-expressions with `[ ]` and `{ }`
lists, `#;`, nested `#| |#`, strings, byte strings, here strings, regexps,
characters, keywords, symbols with `|` and `\`, quote, quasiquote, syntax
and unsyntax prefixes, vectors, hash tables, prefab structs, boxes and
graph labels, and `@`-forms (in `#lang at-exp` code, and with text bodies
in Scribble documents). Any input **shall** be read in time linear in its
size without a panic.

## Rationale

The vendored Racket grammar parses well (none of rebellion's 186 and
typed-racket-lib's 281 files fail, 21 of drracket's 257, DrRacket's WXME
files) but takes 3.6 to 5.8 ms per file and does not read Scribble's
`@`-forms; the reader takes about 0.1 ms per file.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   are read without a panic within the time bound.
2. The reader test's datums read back as written.
