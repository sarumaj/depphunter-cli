---
id: REQ-SWIFT-014
title: Read by a scanner, not the grammar
scope: swift
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Swift sources **shall** be read by a token scanner, not by a tree-sitter
grammar: nested `/* */` comments, string literals (multi-line `"""`, raw
`#"..."#`, with the code of each `\(...)` interpolation read as code), regex
literals (`#/.../#`, and `/.../` where `/` cannot divide), `#if` lines,
attributes and backticked identifiers are tokens. The scanner **shall**
return a result for any input.

## Rationale

The vendored tree-sitter Swift grammar parsed 574 of 3785 files of
swift-argument-parser, vapor, CodeEdit, Lookin and Signal-iOS with errors, and
a file with errors sent the parser into a retry ladder: 180 files ran to the
3-second parse bound (REQ-LANG-011), 1019 s of parsing in all, while the
runtime's memory checks stopped the other workers. Whole analyses took 8.9 s
(swift-argument-parser), 11.6 s (vapor), 10.1 s (CodeEdit), 3.8 s (Lookin)
and 241 s (Signal-iOS); with the scanner they take 0.4 to 0.6 s and 2.5 s.
The scanner reads the same files in about 3 s of CPU (0.2 ms per file at the
median). On the 3211 files the grammar parsed without errors, it gives the
same imports on 99.8% and the same symbols on 99.9% of the files; type uses
differ mostly where the grammar read `Type.member` as a plain name. A panic in
Extract would end the analysis, so the scanner was fuzzed.

## Acceptance criteria

1. Every prefix of a file of unfinished constructs (interpolations, raw
   strings, regex literals, generic arguments, closures, an unterminated
   comment) extracts without an error or a panic.
2. An import in a nested comment or a multi-line string is not read; the
   code of an interpolation in a raw string is.
