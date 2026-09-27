---
id: REQ-LUA-006
title: Rockspecs read as imports
scope: lua
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A rockspec **shall** be read without running Lua, by evaluating its constant
assignments (strings, numbers, booleans, table constructors, `..`
concatenation, local and global variables, fields), and its `dependencies`,
`build_dependencies` and `test_dependencies` (every platform's) **shall** be
imports of the rocks they name, except `lua` itself; its `build.modules`
and `build.install.lua` entries (every platform's) **shall** be imports of
the files that provide them.

## Rationale

A rockspec is Lua, but in practice a table literal with a few variables
(`version = MODREV .. SPECREV`). The dependencies it declares are what a
rock's graph needs even where no file requires them yet.

## Acceptance criteria

1. `telescope.nvim`'s form (`local MODREV, SPECREV = 'scm', '-1'`,
   `version = MODREV .. SPECREV`) reads version `scm-1`.
2. The fixture rockspec imports penlight, luasocket, lua-cjson, lpeg, the
   Unix-only luaposix and the test dependency busted, not lua, and its three
   modules.
