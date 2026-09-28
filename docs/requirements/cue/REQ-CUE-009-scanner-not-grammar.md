---
id: REQ-CUE-009
title: Read by a lexer, not the grammar
scope: cue
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

CUE **shall** be read by a lexer of its own (comments, quoted,
multi-line and `#`-delimited raw strings with interpolations) with
brackets matched once and declarations split where CUE inserts commas,
not a tree-sitter grammar, and any input **shall** be read in time linear
in its size without a panic, and so **shall** the module file.

## Rationale

The vendored tree-sitter CUE grammar parsed 1,091 files of hof,
addreas/homelab, cue-by-example and holos with 21 error files at 1.8-3.8
ms per file; the plugin needs only the top level.

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   are read without a panic within the time bound.
