---
id: REQ-OBJC-013
uuid: ae886a54-317c-46e4-bcaa-a4befae456eb
title: Read by a scanner, not the grammar
scope: objc
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Objective-C declarations **shall** be read by a token scanner (comments,
directives, `@"..."`, character and C++ raw string literals as tokens), not by
the vendored tree-sitter grammar, and the scanner **shall** return a result for
any input.

## Rationale

The vendored Objective-C grammar parsed 317 files of AFNetworking,
SDWebImage and Lookin at 19-36 ms per file (up to 820 ms for one), with ERROR
nodes in 16 of them (5%), some starting at line 1 and swallowing the whole
file. The scanner read 2109 `.m`/`.mm`/`.h` files of six repositories in
0.3 s (about 0.15 ms per file), giving 15816 methods for 16103 method lines
(declarations and definitions of one method count once). A panic in Extract
would end the analysis, so the scanner was fuzzed.

## Acceptance criteria

1. Every prefix of a header and a set of cut constructs extract without an
   error or a panic.

## Notes

Macros are not expanded, so a declaration built by a macro is not seen, and
both branches of an `#if` other than `#if 0` are read.
