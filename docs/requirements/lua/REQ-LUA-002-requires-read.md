---
id: REQ-LUA-002
title: Requires read
scope: lua
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Lua plugin **shall** record as imports: `require` with a string literal
argument in every call form (`require("a.b")`, `require "a.b"`,
`require 'a.b'`, `require [[a.b]]`), `pcall(require, "a.b")`, Luau's
requires by path (`"./x"`, `"../x"`, `"@self/x"`, `"@alias/x"`), Roblox's
requires of an instance (`script`, `game`, `workspace` or a local variable
holding one of them, followed by `.Name`, `.Parent`, `["Name"]`,
`:WaitForChild("Name")`, `:FindFirstChild("Name")` or
`game:GetService("Name")`), and `dofile` and `loadfile` of a string
literal. A require whose argument is computed **shall not** be recorded. The
import's spec is the call as written.

## Rationale

These are the ways Lua, Neovim plugins, LÖVE games, OpenResty applications and
Roblox games load code; a module a program computes at run time cannot be
known from its text.

## Acceptance criteria

1. The fixture's `src/shop/init.lua` records the string forms,
   `pcall(require, "cjson")`, `require [[shop.native]]`, `dofile` and
   `loadfile`, and not `require(cart.name)`.
2. `game/src/shared/Game/init.luau` records instance requires through
   `script`, a `game:GetService` local and a local holding `script.Parent`,
   and Luau string requires.
3. A require inside a long string, a long comment or a Luau interpolated
   string is not read.
