---
id: REQ-SOLIDITY-009
title: Islands
scope: solidity
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** declare three islands: `soldeer` ("Soldeer
packages"), `git-submodule` ("Git submodules") and `npm` (JavaScript's,
same id and name), and emit no other. `soldeer` and `git-submodule`
**shall** be private-pattern prefixes (REQ-SUP-035); neither is asked
about by OSV or Trivy, nor by an index with `--online`.

## Rationale

Git submodules are a git mechanism, not a Solidity one, so their island is
named for git and other languages may share it. OSV has no Solidity
ecosystem; Soldeer's registry API is not reachable from here and says
nothing about dependencies.

## Acceptance criteria

1. The plugin declares exactly these islands, none a standard library.
