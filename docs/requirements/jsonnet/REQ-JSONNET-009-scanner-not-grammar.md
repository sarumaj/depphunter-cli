---
id: REQ-JSONNET-009
title: Read by a lexer, not the grammar
scope: jsonnet
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Jsonnet **shall** be read by a lexer of its own (comments, escaped and
verbatim strings, `|||` text blocks) with brackets matched once, not a
tree-sitter grammar, and any input **shall** be read in time linear in its
size without a panic.

## Rationale

The vendored tree-sitter Jsonnet grammar parsed 1,470 files of
kube-prometheus, mimir's operations/ and grafana/jsonnet-libs with 11
error files at 2.5-3.7 ms per file; imports and the top-level object need
only tokens, which the lexer reads far faster.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   are read without a panic within the time bound.
