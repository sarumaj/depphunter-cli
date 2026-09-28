---
id: REQ-SOLIDITY-007
title: Soldeer dependencies
scope: solidity
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The `[dependencies]` of `foundry.toml` (a version string, or a table with
`version` and `git` with `rev`, `tag` or `branch`, or `url`) and the
`[[dependencies]]` of the `soldeer.lock` beside it (`name`, `version`,
`url` or the older `source`, `git`, `rev`, `checksum`) **shall** be
packages of the `soldeer` island named by their dependency name, and a
path into `dependencies/<name>-<version>/` beside the `foundry.toml` the
package whose name it starts with. The lock **shall** pin a package (its
`rev` as version for a git source, the declared version as requested when
it differs); without it an exact version pins, a git `rev` that is a full
commit pins with the version as requested, a tag is shown and neither
pins nor floats, and a branch, a git source without a ref or a version
requirement (`^1.0.0`) floats. A git source off the public forges or a
`url` source **shall** be the origin. For `--resolve-depth`, a package
installed into `dependencies/<name>-<version>/` (the directory of its
version first, else one of its name) **shall** depend on the
`[dependencies]` of the `foundry.toml` and the entries of the
`soldeer.lock` inside that directory, each pinned as the project pins it
when the project names it too, else as those files do, and be reported as
installed; unreadable files there say nothing.

## Rationale

The lock records exactly what Soldeer installed; the registry's releases
are immutable uploads, so an exact version names one of them. What a
package depends on is only in the files it ships.

## Acceptance criteria

1. The fixture's `foundry.toml` and `soldeer.lock` entries and the sources'
   `@openzeppelin-contracts-5.0.2/...` and `solady-0.0.200/...` imports
   resolve as listed.
2. The installed `@openzeppelin-contracts-5.0.2` depends on forge-std at
   the project's locked 1.9.1 and on solady at its own locked 0.0.227;
   garbage manifests there give no dependencies.
