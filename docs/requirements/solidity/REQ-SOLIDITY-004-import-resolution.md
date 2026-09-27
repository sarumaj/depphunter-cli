---
id: REQ-SOLIDITY-004
title: Import resolution
scope: solidity
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An import path **shall** resolve in this order: a path starting `./` or
`../` is the file it names from the importing file; else the project's
remappings (REQ-SOLIDITY-005) map it; else an npm package a `package.json`
above the file declares is that package (REQ-SOLIDITY-008); else a file at
that path from the project's root (the nearest directory with a
`foundry.toml`, a `hardhat.config.*` or a `remappings.txt`), then from the
repository's root, is that file. What a path reaches **shall** be: a
package when it lies in a git submodule (REQ-SOLIDITY-006), in a
`node_modules` package, in a library of a Foundry libs directory, or in a
Soldeer dependency (REQ-SOLIDITY-007), even when the dependency's files
are on disk; else the project's file or directory. A path nothing
resolves **shall** be dropped when it leaves the repository or names a
directory of the project, and otherwise be an unresolved package named by
its first segment: an npm package (`@scope/name` for a scope) outside a
Foundry project and for a scope, a git submodule in a Foundry project.

## Rationale

This is the order solc (with Foundry's base path) and Hardhat look in. A
library is an external package whether or not its submodule is checked
out, so its files never become the project's.

## Acceptance criteria

1. The fixture's Foundry, Hardhat and plain sources resolve as listed,
   including dropped relative and remapped paths and a missing file of the
   project's `script/` directory.
