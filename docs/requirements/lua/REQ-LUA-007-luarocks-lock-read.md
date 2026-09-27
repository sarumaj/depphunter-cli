---
id: REQ-LUA-007
title: luarocks.lock read
scope: lua
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The `luarocks.lock` beside a rockspec (else the project root's), read from
disk when it is not scanned, **shall** pin the rocks it lists to their locked
version with revision (`1.14.0-3`), and a rock only the lock lists **shall**
count as declared (it was installed as a dependency of one).

## Rationale

`luarocks build --pin` writes the exact versions installed; a module of a
rock installed only transitively (luafilesystem through penlight) is still
installed.

## Acceptance criteria

1. `penlight ~> 1.5` locked to `1.14.0-3` resolves pinned at `1.14.0-3`
   with `~> 1.5` requested.
2. `require("lfs")` resolves to the lock-only luafilesystem, pinned.
