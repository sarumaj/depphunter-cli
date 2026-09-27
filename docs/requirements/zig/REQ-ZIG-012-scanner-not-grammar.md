---
id: REQ-ZIG-012
title: Read by a scanner, not the grammar
scope: zig
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Zig sources and manifests **shall** be read by a lexer of their own, not a
tree-sitter grammar: line comments, strings with escapes that end at the
line, multi-line `\\` string lines, character literals (`'"'`), `@"quoted"`
identifiers, builtins, numbers with exponents told from ranges (`0..5`).
Brackets are matched once; the reader, the ZON parser and the build-code
evaluator **shall** return a result for any input in time linear in its
size, with bounded nesting and look-ahead.

## Rationale

The vendored grammar was measured on zls, libxev, tigerbeetle, ghostty and
lib/std of Zig: 8 to 14 ms per file (360 ms for ghostty's Terminal.zig),
17.5 s for all 1727 files, and ERROR nodes in up to 36 of 550 files
(lib/std: inline assembly and newer syntax). The reader extracts the same
files in about 0.4 s. A panic in Extract would end the analysis, so it was
fuzzed.

## Acceptance criteria

1. Every prefix of every fixture file and runs of brackets, struct
   literals, containers, functions, quotes and build calls extract without
   a panic.
2. Imports after a string holding `@import("x")`, a `\\` line or a
   `'"'` literal are read, and none inside them.
