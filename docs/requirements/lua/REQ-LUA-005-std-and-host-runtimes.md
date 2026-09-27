---
id: REQ-LUA-005
uuid: 12604cce-18a2-475b-aea9-4201257927b3
title: Standard library and host runtimes
scope: lua
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A require of the standard library or of LuaJIT's built-in modules
(`string`, `table`, `math`, `io`, `os`, `coroutine`, `debug`, `utf8`,
`package`, `bit32`, `jit`, `ffi`, `bit` and their submodules, such as
`jit.opt` or `table.new`) **shall** resolve to the hidden `lua-std` island
named by its first segment, unless a rockspec declares a rock of that name.
A module a host program provides **shall** resolve to the hidden
`lua-runtime` island named by the host: `vim.*` to `neovim`, `love.*` to
`love2d`, `ngx.*`, `ndk.*` and the lua-resty libraries OpenResty bundles to
`openresty` (and `cjson` too in a project that declares lua-resty rocks),
and Luau's `@lune/*` to `lune`. Neovim's `vim` global itself is not an
import.

## Rationale

These modules come with the interpreter or the program embedding it; no rock
installs them, and a project would otherwise be charged with dozens of
undeclared "rocks". One node per host keeps the island small.

## Acceptance criteria

1. `string` and `jit.opt` resolve to lua-std `string` and `jit`.
2. `vim.lsp`, `love.graphics`, `ngx.ssl` and `@lune/fs` resolve to
   lua-runtime `neovim`, `love2d`, `openresty` and `lune`.
