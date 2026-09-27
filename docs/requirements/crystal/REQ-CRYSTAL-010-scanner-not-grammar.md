---
id: REQ-CRYSTAL-010
title: Read by a scanner, not the grammar
scope: crystal
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Crystal **shall** be read by a lexer of its own, not a tree-sitter grammar:
`#` comments, strings with escapes and nested `#{}` interpolations,
backtick commands, heredocs, %-literals with nesting delimiters (`%q` with
no escapes), regexes told from division by the previous token, char
literals, symbols, labels, macro tags and expressions as opaque tokens, and
`end`-closed blocks with modifiers (`x if y`) told apart. Any input **shall**
be read in time linear in its size without a panic. The shards files are
read with gopkg.in/yaml.v3.

## Rationale

The vendored grammar took 6 to 25 ms per file and parsed 259 of 715 files of
kemal, ameba, lucky and Crystal's json/ and http/ with errors;
everything needed is token-level.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   extract without a panic within the time bound.
