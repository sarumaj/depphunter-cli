---
id: REQ-LUA-012
title: Read by a scanner, not the grammar
scope: lua
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Lua, Luau and Teal sources and LuaRocks' Lua files **shall** be read by a
token scanner, not by a tree-sitter grammar: long strings and comments of any
level, escapes (`\z`, `\x`, `\u{}`, decimal), a byte order mark and a
first `#` line, numerals (hex floats, Luau's `0b` and `_`, LuaJIT's
suffixes) and Luau interpolated strings (their `{...}` code lexed so nested
strings and braces do not end them) are tokens. The scanner and the constant
evaluator **shall** return a result for any input.

## Rationale

The vendored grammars were measured on Penlight, telescope.nvim, Kong, matter,
roact and Knit: the Lua grammar took 3-5 ms per file (6 s for Kong's 1309
files, up to 102 ms for one file) and parsed 26 of matter's 61 Roblox `.lua`
files with errors (they are Luau); the Luau grammar parsed those cleanly but
would have to be chosen by content. The scanner reads Kong's 10 MB of Lua in
about 0.35 s of CPU, and needs nothing but what a require, a definition and
a block boundary look like. A panic in Extract would end the analysis, so the
scanner was fuzzed.

## Acceptance criteria

1. Every prefix of every fixture file, and inputs cut inside each construct
   (long brackets, escapes, interpolations, require calls, type
   annotations), extract without an error or a panic; 100000 nested brackets
   do not exhaust the stack.
2. A require after long strings, long comments, escapes, an interpolated
   string and an if-expression is read on its own line.
