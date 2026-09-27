---
id: REQ-LUA-001
uuid: 44ff1b8d-699e-400b-955f-f2bf4b7a5869
title: Lua, Luau and Teal files and their manifests claimed
scope: lua
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Lua plugin **shall** claim Lua (`.lua`), Luau (`.luau`) and Teal
(`.tl`) sources, rockspecs (`.rockspec`), Wally manifests (`wally.toml`)
and Rojo project files (`*.project.json`), and **shall not** claim what
LuaRocks and Wally install into a project (`lua_modules`, `.luarocks`,
`Packages`, `DevPackages`, `ServerPackages`). `wally.toml` and Rojo
projects, which share their extensions with other formats, have their own
extraction cache classes. `.luau` files are labelled Luau, `.tl` files Teal,
rockspecs and `luarocks.lock` Lua, and a walk without git skips `lua_modules`
(REQ-LANG-018).

## Rationale

Lua code is plain Lua, LuaJIT, Roblox's Luau (whose `.lua` files are Luau
too) or Teal; a rock's dependencies are declared in its rockspec, a Roblox
project's in `wally.toml`, and where its code lands in the game in a Rojo
project. An installed tree is somebody else's code.

## Acceptance criteria

1. `a.lua`, `b.luau`, `c.tl`, `shop-1.0-1.rockspec`, `wally.toml`,
   `default.project.json` and `place.project.json` are claimed;
   `project.json`, `Cargo.toml`, `lua_modules/.../x.lua` and
   `Packages/Roact.lua` are not.
2. `wally.toml` has cache class `wally`, a Rojo project `rojo`.
