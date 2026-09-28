---
id: REQ-NIM-004
title: Modules found as the compiler finds them
scope: nim
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An import or include **shall** resolve to a file of the repository as the
compiler finds it: `./x` and `../x` only beside the importing file (else
dropped); another path beside the importing file, then under the package's
`srcDir` (from its `.nimble`; the package's directory without one), then
under the search paths the `nim.cfg`, `<name>.nim.cfg`, `config.nims` and
`<name>.nims` files of the file's directory and those above it add
(`--path:x`, `path = "x"`, `switch("path", thisDir() / "x")`, `$projectDir`
read as the configuration's directory); a module name in either case as
written or in lower case. A path with an extension other than `.nim`
(`include "nimble.paths"`) names that file. The search paths **shall** also
be imports of their configurations, edges to their directories (or to the
installed package a path is, REQ-NIM-007).

## Rationale

Nim has no module-to-package manifest: the compiler searches directories.

## Acceptance criteria

1. The fixture's `vlib`, `generated` and `helper` resolve through
   `config.nims` and `nim.cfg`, `shop` from `tests/` through the package's
   `srcDir`, and `./nothere` is dropped.
