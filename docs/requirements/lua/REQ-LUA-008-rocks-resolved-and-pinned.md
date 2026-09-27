---
id: REQ-LUA-008
title: Rocks resolved and pinned
scope: lua
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A module no project file provides **shall** resolve to the rock the governing
rockspecs (in the requiring file's directory and above, else all) declare or
their locks hold, matched by, in order: a curated alias table (`lfs` ->
luafilesystem, `socket`/`mime`/`ltn12` -> luasocket, `ssl` -> luasec,
`cjson` -> lua-cjson, `posix` -> luaposix, `re` -> lpeg, `pl` ->
penlight, `lxp` -> luaexpat and others), `lua-resty-<x>` for `resty.<x>`,
and the module's first segment as the rock's name, `lua-<x>`, `<x>-lua`,
`lua<x>`, `<x>.nvim` or `nvim-<x>` (compared without case, `-`, `_` and
`.`), or a fork named `<owner>-<x>` (kong-pgmoon for `pgmoon`). Otherwise
it **shall** be an unresolved luarocks package named by the alias table or its
first segment. Pinning: a lock pins; `== 1.2.3` or a bare version (LuaRocks
reads it as `==`) pins with the version shown bare; any other constraint
(`~>`, `>=`, a list) is kept as the version and floats; no constraint
floats.

## Rationale

Most rocks provide modules named after them; the exceptions are well known
and few. LuaRocks' own rule makes a bare version exact.

## Acceptance criteria

1. `pl.utils` resolves to penlight, `socket.http` to the declared luasocket
   (floating), `pcall(require, "cjson")` to lua-cjson 2.1.0 (pinned).
2. `ssl` resolves to an unresolved luasec, `resty.http` to an unresolved
   lua-resty-http, `unknownmod.x` to an unresolved unknownmod,
   `plenary.async` to an unresolved plenary.nvim.
3. `luaposix >= 35` keeps its constraint and is not pinned.
