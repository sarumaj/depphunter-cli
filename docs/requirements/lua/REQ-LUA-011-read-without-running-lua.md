---
id: REQ-LUA-011
title: Read without running Lua, LuaRocks or Rojo
scope: lua
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Lua is read without running Lua, LuaRocks, Wally or Rojo: `package.path` and
`package.cpath` set at run time (`LUA_PATH`, a script's own
`package.path = ...`) are not read, so modules are found under conventional
roots; requires of computed names, requires through `require` stored in
another variable, and Roblox instances created or moved at run time are not
followed; a module no rockspec declares is matched to a rock by a curated
table and name heuristics; C modules without a rockspec entry are not
found; `luarocks.lock` has no rock-to-rock edges, so only `--online` walks
past the first level of rocks; and no vulnerability database covers LuaRocks
or Wally (OSV has neither ecosystem).

## Rationale

Evaluating Lua or building the Roblox place would mean running the
project's code.

## Acceptance criteria

1. `require(cart.name)` is not recorded, and `ssl`, which no rockspec declares,
   is named luasec by the alias table and left unresolved.
2. The limitation is listed under "Known limitations" in the requirements
   README.
