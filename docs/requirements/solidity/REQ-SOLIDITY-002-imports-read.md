---
id: REQ-SOLIDITY-002
title: Imports read
scope: solidity
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read every import directive of a source unit in all
of its forms (`import "p";`, `import "p" as X;`, `import * as X from
"p";`, `import {A, B as C} from "p";`), once per path per file, with the
path as written. A `pragma` is not an import. Nothing in a comment (`//`,
`/* */`, NatSpec `///` and `/** */`), a string (single or double quoted,
with escapes, `unicode"..."`, `hex"..."`), a function or modifier body or
an `assembly` block **shall** be read as an import, and a missing `;`
**shall** not hide the next import. Each remapping of `foundry.toml` and
`remappings.txt`, each Soldeer dependency of `foundry.toml`, each entry
of `soldeer.lock` and each submodule of `.gitmodules` **shall** be an
import of its file.

## Rationale

Imports are only allowed at the source unit's level, so reading that level
alone keeps what code and data mention out of the graph. The manifests'
entries are edges even for dependencies no source imports yet.

## Acceptance criteria

1. The import-form test's sources give exactly the listed paths.
2. The fixture's `Counter.sol` fake imports in NatSpec, a block comment, a
   string and an assembly block are not imports.
