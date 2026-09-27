---
id: REQ-SOLIDITY-001
title: Files claimed
scope: solidity
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Solidity plugin **shall** claim Solidity sources (`.sol`), Foundry's
`foundry.toml` and `remappings.txt`, Soldeer's `soldeer.lock`, and a
`.gitmodules` beside a `foundry.toml` (the manifests told apart from other
files by their names). Nothing in a `node_modules` directory, in a `lib`,
`dependencies`, `out` or `cache` directory beside a `foundry.toml` (the
libraries Foundry installed, Soldeer's dependencies, forge's build output),
or in an `artifacts`, `cache` or `typechain-types` directory beside a
`hardhat.config.*` (Hardhat's build output and TypeChain's bindings)
**shall** be claimed, and a walk of the file system **shall** not enter
them. The scan **shall** label `.sol` files Solidity, `foundry.toml` and
`remappings.txt` Foundry and `soldeer.lock` Soldeer.

## Rationale

A Foundry project's `lib/` holds its dependencies' sources (git submodules
or vendored copies), not its own code; git lists a submodule only as a
commit, so in git mode its files are not listed anyway.

## Acceptance criteria

1. The fixture's sources and manifests are claimed; its
   `lib/forge-std/src/Test.sol`, `dependencies/...`, `out/...`,
   `cache/...`, `artifacts/...` and `typechain-types/...` files are not,
   nor is a `.gitmodules` without a `foundry.toml` beside it.
