---
id: REQ-SOLIDITY-008
title: npm packages shared with JavaScript
scope: solidity
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An import of an npm package (Hardhat's `@openzeppelin/contracts/...`,
`hardhat/console.sol`), directly or through a remapping into
`node_modules`, **shall** resolve through the JavaScript plugin's reading
of `package.json` and its lock files: a workspace package of the
repository is its file or directory, a package a `package.json` above the
file declares is an `npm` package with the version and pin JavaScript's
imports get (REQ-JS-006, REQ-JS-007, REQ-JS-010), and its dependencies for
`--resolve-depth` are the lock files'. So a Solidity import and a
JavaScript import of one package **shall** be one node.

## Rationale

Hardhat projects install their Solidity libraries with npm; one package
must not appear twice because two languages import it.

## Acceptance criteria

1. The fixture's Hardhat imports resolve to npm packages pinned by
   `package-lock.json`.
2. A Hardhat project whose contract and deployment script both import
   `@openzeppelin/contracts` has one npm node, version 5.0.2, with an edge
   from each file.
