---
id: REQ-CMAKE-002
uuid: 8c55e79e-01ec-4471-bb03-7e1adf226c17
title: CMake files read by a scanner
scope: cmake
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

CMake files **shall** be read as command invocations `name(arguments)` with
command names in any case, `#` line comments and `#[[ ]]` bracket comments
skipped, bracket arguments `[=[ ]=]` taken verbatim, quoted arguments with
their escapes and line continuations decoded, unquoted arguments split into
list elements at unescaped `;`, and parentheses nested in the arguments kept
as arguments. An unterminated construct **shall** end the file, not the
analysis.

## Rationale

The whole language is command invocations; everything the plugin needs is at
the level of commands and their arguments.

## Acceptance criteria

1. A command in a bracket comment or a `#` comment is not read.
2. `Add_SubDirectory ( src )` is `add_subdirectory` with the argument `src`.
3. A quoted argument `x\"y\`, a line break, and `z` is one argument
   `x"y z`; `b\;c` is not split.
4. Truncated input (`f("abc`, `f([[abc`) is read without an error.
