---
id: REQ-LUA-010
uuid: baa87a29-a443-4db8-9df4-bb3bacee593d
title: Luau and Roblox requires resolved
scope: lua
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A Luau require by path **shall** resolve `./x` and `../x` from the
requiring file's directory (from an `init` file's parent directory, since
the init file stands for its directory), `@self/x` from the module itself,
and `@alias/x` through the aliases of the nearest `.luaurc` above the file
(relative to that file), probing `x`, `x.luau`, `x.lua` and `x/init.*`.
A Roblox instance require **shall** resolve as Rojo builds the game:
`script` is the requiring file (an init file is its directory), `.Parent`
the parent instance, a child the file or directory of that name; a Rojo
place project (a `*.project.json` whose tree is a `DataModel`,
`default.project.json` first) maps `game.<Service>...` instances to its
`$path`s and back, and a `$path` naming a directory with a model
`default.project.json` builds that project's tree. The `$path`s of a Rojo
project **shall** be imports of the files and directories they name. What
cannot be placed **shall** be dropped.

## Rationale

Roblox code requires instances, not files; Rojo's project files are the only
record of where a file ends up in the game.

## Acceptance criteria

1. From `game/src/shared/Game/init.luau`: `script.Parent.Util`,
   `Shared.Util` (a local holding `script.Parent`) and `"./Util"` resolve
   to `game/src/shared/Util.lua`; `script.Child` and `"@self/Child"` to
   `Child.luau`; `ReplicatedStorage.Shared.Types` and `"@Shared/Types"` to
   `Types.luau`; `script.Parent.Nope` and `"../missing"` are dropped.
2. The project's `$path: src/shared` imports the directory; `$path:
   Packages` (not committed) is dropped.
