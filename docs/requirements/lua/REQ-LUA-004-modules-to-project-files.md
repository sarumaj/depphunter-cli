---
id: REQ-LUA-004
title: Modules resolved to project files
scope: lua
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A required module name **shall** resolve, in order, to the project file a
rockspec's `build.modules` or `build.install.lua` maps it to (the
rockspecs in the requiring file's directory and above first, then any;
paths relative to the rockspec, else to the project root; a C module to its
first source), to the file `package.path`'s usual templates (`?.lua`,
`?/init.lua`, and `.luau` and `.tl` alike, dots read as directories)
find under the requiring file's directory, each directory above it up to the
root, and their `lua`, `src` and `lib` subdirectories (a Neovim plugin's
modules live in `lua/`), then under the directories of a `.luarc.json`'s
`runtime.path` templates and relative `workspace.library` entries, and, when
nothing else knows the module, to the one project file whose path ends in the
module's path. A module of the project's own rock that is not in the
checkout **shall** be dropped rather than named as a rock. Files under
`lua_modules`, `.luarocks` and Wally's package folders are never the
answer.

## Rationale

`package.path` is set at run time by whoever runs the code; rockspecs say
exactly which file provides which module, and the conventional roots cover
applications, libraries and Neovim plugins.

## Acceptance criteria

1. `shop.cart` resolves to `src/shop/cart.lua` through `build.modules`,
   `shop.native` to `csrc/native.c`, `shop.util` to `src/shop/util.lua`
   through `src/`, `myplugin.config` to
   `nvim/myplugin/lua/myplugin/config.lua`, and `extra.mod` to the
   `.luarc.json` library although another `extra/mod.lua` exists.
2. `dofile("scripts/setup.lua")` resolves to the file; a missing one is
   dropped.
