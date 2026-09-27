---
id: REQ-ZIG-002
uuid: 52919ed3-95dc-47db-8559-8510ddfe424b
title: Imports read
scope: zig
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read, anywhere in a Zig file, `@import`, `@embedFile`
and `@cInclude` (inside `@cImport`) with a string literal argument, and in
build code (a file using `std.Build`) `b.path("x")` (also `builder.path`),
Zig 0.11's `.{ .path = "x" }` and `b.dependency("x", ...)` or
`b.lazyDependency("x", ...)`. The spec **shall** be the call as written
(`@import("std")`, `b.path("src/main.zig")`); one spec **shall** be one
import per file. Strings, multi-line `\\` strings, character literals and
comments **shall not** yield imports.

## Rationale

Zig names everything a file uses with builtins taking literal strings, so
the imports are exact without evaluating code.

## Acceptance criteria

1. `src/main.zig` of the fixture yields its `@import`s, the
   `@embedFile` and both `@cInclude`s; `build.zig` yields each `b.path`
   and dependency.
2. An `@import` inside a string, a `\\` line or a comment is not read.
