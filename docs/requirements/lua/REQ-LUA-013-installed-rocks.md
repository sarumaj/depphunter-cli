---
id: REQ-LUA-013
title: Installed rocks' dependencies
scope: lua
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** read the LuaRocks trees of the repository
(`lua_modules/` and `.luarocks/` at the root and beside each rockspec) from
disk: `lib/luarocks/rocks-<lua version>/<rock>/<version>/` with the
`<rock>-<version>.rockspec` LuaRocks keeps there, the newest version of a
rock per tree. `--resolve-depth` **shall** follow, for a rock a tree
holds, the `dependencies` of that rockspec (not `build_dependencies`,
`test_dependencies` or `lua`), each pinned to the version the same tree
holds (the constraint as requested), else as its constraint says, and
report them as installed.

## Rationale

`luarocks.lock` is a flat list; the rockspecs LuaRocks installs beside each
rock record its dependencies.

## Acceptance criteria

1. The installed lua-resty-http depends on lua-resty-openssl at the tree's
   newest 0.10.0-1 (requested `>= 0.9`) and on a floating luasocket, not on
   lua or busted; luasocket, in the `.luarocks` tree, depends on nothing; a
   rock no tree holds is not answered; a garbage rockspec names nothing.
