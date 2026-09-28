---
id: REQ-NIM-002
title: Imports read
scope: nim
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `import`, `from ... import` and `include`
statements at a module's top level, also inside `when`, `elif` and `else`
branches, all of which are read (conditions are not evaluated), in every
form: lists (`import a, b/c`) continued on indented lines, bracket groups
(`std/[os, strutils]`, `compiler / [ast, idents]`), `pkg/x`, `./x` and
`../x`, quoted paths (`"x.nim"`), `as` aliases and `except` lists; one
import per module and kind per file. Imports in comments (`#`, `##`, nested
`#[ ]#` and `##[ ]##`), in strings (`"..."`, raw `r"..."` and generalized
raw `fmt"..."` strings, `"""..."""`) and in routine bodies **shall not** be
read; `export` statements are not imports.

## Rationale

Imports are the module graph's edges; Nim allows them only at the top level
(a `when` branch is still the top level).

## Acceptance criteria

1. The fixture's `src/shop.nim` imports exactly the modules listed in the
   test, including those in a `when defined(windows)` branch, its `else`
   and an inline `elif`, and none of the fake imports in its comments and
   strings.
