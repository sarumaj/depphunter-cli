---
id: REQ-SOLIDITY-005
title: Remappings
scope: solidity
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A project's remappings **shall** be those of `foundry.toml` (every
profile's `remappings`, the default profile's first), then those of
`remappings.txt` (comments and blank lines skipped), then, for a remapping
prefix neither lists, the ones Soldeer generates for its dependencies
(`[soldeer] remappings_prefix` + `name-version/` = `dependencies/name-version/`,
without the version when `remappings_version` is false) and, unless
`auto_detect_remappings` is false, the ones Foundry infers for each
library in the default profile's `libs` (default `lib`): `dep/` =
`lib/dep/src/` when the library has a `src` directory on disk, else
`lib/dep/`, the libraries being the submodules git records there and the
directories on disk, and one level of the libraries' own `lib/`
directories likewise. A remapping `context:prefix=target` **shall** apply
to an import of a file whose path from the project's root starts with the
context; of those that apply, the one with the longest context, then the
longest prefix, then the first listed wins, and the import path's prefix is
replaced by the target.

## Rationale

These are solc's rules and Foundry's and Soldeer's defaults; without the
inferred remappings `forge-std/Test.sol` resolves nowhere in a project that
never writes one.

## Acceptance criteria

1. The remap test's context beats a longer prefix and a prefix without a
   trailing slash maps as written.
2. The fixture's `util/` imports reach `src/utils` from `src/` and
   `test/helpers` from `test/`; `vault/` reaches the vault submodule
   through the inferred remapping; Soldeer's `name-version/` paths reach
   their packages.
