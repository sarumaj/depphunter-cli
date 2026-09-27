---
id: REQ-CLOJURE-011
title: Read by a reader, not the grammar
scope: clojure
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Clojure sources and manifests **shall** be read by a reader of their own
(`internal/lang/edn`), not a tree-sitter grammar: lists, vectors, maps,
sets, strings, character literals (`\(`, `\newline`), regexes, comments,
`#!` lines, quote, syntax quote, unquote, deref and var quote, metadata
(dropped), `#_` (discarding the next form, repeatable), reader
conditionals (every branch spliced), namespaced maps `#:ns{}`, tagged
literals and `##Inf`. The reader **shall** return a result for any input in
time linear in its size: unbalanced closers are ignored, input ending
inside forms closes them, nesting and pending prefixes are bounded.

## Rationale

The vendored grammar was measured on re-frame, compojure, ring, babashka
and metabase: 3.7 to 6.6 ms per file, 28 s of parsing for metabase's 4253
files (one file with an ERROR node); the reader reads all of metabase in
well under a second. A panic in Extract would end the analysis, so the
reader was fuzzed.

## Acceptance criteria

1. Every prefix of every fixture file and 1 MB runs of brackets, quotes,
   metadata, `#_`, reader conditionals, strings and backslashes read
   without a panic.
2. `(a #_ #_ b c d)` reads as `(a d)`, and `(r #?@(:clj [a b] :cljs [c]))`
   as `(r a b c)`.
