---
id: REQ-SOLIDITY-011
title: Read without the tools
scope: solidity
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Solidity projects **shall** be read without running solc, forge, Soldeer or
Hardhat: remappings set by environment variables or on a command line, a
`hardhat.config.*`'s own paths and remappings, Foundry's inference beyond
`lib/<dep>/src` and one nested level, and a profile's `libs` other than
the default's are not known; the recorded submodule commit comes from git's
index (without git it is unknown and the submodule floats); what a
submodule's checkout imports is not read (its files are never the
project's); a Soldeer package's own dependencies are read only from what
Soldeer installed at the top of `dependencies/` (not its nested
`dependencies/`), and nothing is asked of Soldeer's registry.

## Rationale

The tools' own resolution depends on the environment they run in.

## Acceptance criteria

1. Without a gitlink for a submodule, the submodule floats.
