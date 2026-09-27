---
id: REQ-LUA-003
uuid: 1c6db901-2b88-48dc-9193-3fd59eb69906
title: Definitions extracted
scope: lua
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Lua plugin **shall** extract the definitions a file makes outside any
function body: named functions (`function f`, `local function f`,
`function a.b.c` as `a.b.c`), methods (`function a.b:c` as `a.b.c`, and
`a.b = function(self, ...)`), functions assigned to a global or field
(`M.f = function`), module tables (`local M = {}`, `M = {}`,
`M.sub = {}`, `setmetatable(...)`), other values assigned to a field
(`M.version = "1"`, kind `var`), Luau's `type` and `export type` and
Teal's records (class), interfaces, enums and types. A name assigned again at
the top level (a local set in an `if`, a field reassigned) **shall not** be
another definition. Luau if-expressions, which have no `end`, and Teal
function types **shall not** be taken for blocks.

## Rationale

A Lua module is a table of functions; its top-level functions and fields are
what other files use. Functions nested in other functions are local helpers.

## Acceptance criteria

1. `src/shop/init.lua` yields `M`, `M.version`, `M.add`, `M.checkout`
   (method), `M.handler` (method), `helper` and `M.maybe` (inside a
   top-level `if`), and not `nested` or `inner`.
2. `Types.luau` yields `Item` and `Pair` (types) and `M.after` after an
   if-expression; `shapes.tl` yields `Point`, `Color`, `Callback`,
   `make` and `helper` and not the nested record's fields.
