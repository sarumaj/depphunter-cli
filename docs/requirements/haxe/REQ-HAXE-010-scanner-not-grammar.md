---
id: REQ-HAXE-010
title: Read by a scanner, not the grammar
scope: haxe
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Haxe **shall** be read by a lexer and declaration scanner of its own, not a
tree-sitter grammar, and any input **shall** be read in time linear in its
size without a panic.

## Rationale

The vendored grammar parsed 905 of 983 files of tink_core, HaxeFlixel and
Heaps with errors, at 9 to 13 ms per file (343 ms for one).

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   extract without a panic within the time bound.
