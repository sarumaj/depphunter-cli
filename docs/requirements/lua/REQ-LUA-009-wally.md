---
id: REQ-LUA-009
title: Wally packages
scope: lua
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The dependencies of a `wally.toml` (`[dependencies]`,
`[server-dependencies]`, `[dev-dependencies]`: `Alias =
"scope/name@requirement"`) **shall** be imports of Wally packages named
`scope/name`. A require whose instance or file path goes through a
`Packages`, `DevPackages` or `ServerPackages` folder (by name, or a folder
a Rojo project maps to one) **shall** resolve to the package the nearest
`wally.toml` above the file (else any) names by the next element as its
alias. The `wally.lock` beside the manifest **shall** pin; `=1.2.3` pins; a
bare version, which Wally reads as a caret range, and any other requirement
float. `wally.lock`'s per-package dependencies **shall** answer
`--resolve-depth`.

## Rationale

Wally installs a Roblox project's packages into a Packages folder that is not
committed; code reaches them by alias through that folder.

## Acceptance criteria

1. `require(ReplicatedStorage.Packages.Roact)` and
   `require(ReplicatedStorage:WaitForChild("Packages"):WaitForChild("Promise"))`
   resolve to roblox/roact 1.4.4 (requested 1.4.0) and evaera/promise 4.0.0,
   pinned by the lock.
2. The unlocked dev dependency roblox/testez keeps 0.4.1 and is not pinned.
3. roblox/roact's dependencies are evaera/promise 4.0.0 from the lock.
