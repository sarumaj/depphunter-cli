---
id: REQ-CPP-014
title: Read by a scanner, not the grammar
scope: cpp
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

C and C++ definitions **shall** be read by a token scanner, not by a
tree-sitter grammar: a tolerant recursive descent over the file, namespace,
`extern "C"` and class bodies that steps over function bodies, initializers
and bracket groups it does not recognise. Of a conditional group whose
branches do not each balance their braces, only the first branch **shall** be
read. The scanner **shall** return a result for any input in time linear in
its length.

## Rationale

The vendored tree-sitter C and C++ grammars parsed 8606 of 14857 files of
spdlog, libuv, Catch2, flameshot, fmt, nlohmann/json, benchmark, the Arcade
Learning Environment, ModernCppStarter, abseil-cpp, envoy and grpc with
errors, and a file with errors sent the parser into a retry ladder. 83 of
grpc's generated protobuf files (`upbdefs-gen/*.upbdefs.c`, whose long
character-literal tables the C grammar parses in super-linear time, and
`upb-gen/*.upb.h`) each ran to the 3-second parse bound (REQ-LANG-011): 345 s
of parsing for grpc, 78 s for envoy and 17.5 s for abseil-cpp, where no file
reached the bound. Whole analyses took 90 s (grpc), 22 s (envoy) and 5.4 s
(abseil-cpp); with the scanner they take 3.5 s, 3.7 s and 0.5 s, and the
scanner reads all 14857 files in 2 s. On the 6251 files the grammars parsed
without errors, it finds 99.96% of the grammars' symbols with the same name
and kind (99.9% on the same line) and 99.5% of its symbols are the grammars';
the rest are mostly definitions the grammars dropped without reporting an
error (a class behind an export macro, `X::X() = default;`). Includes are
read by the preprocessor scan (REQ-CPP-002) either way, so dependency edges
do not change.

## Acceptance criteria

1. Every prefix of the fixture files, and runs of unfinished constructs
   (brackets, template argument lists, comments, raw strings, constructor
   initializers, conditional groups), extract without an error or a panic.
2. A 200000-element character table extracts its one function in well under
   the parse bound.
3. Alternative function heads in `#ifdef`/`#else` sharing one body leave the
   definitions after them intact.
