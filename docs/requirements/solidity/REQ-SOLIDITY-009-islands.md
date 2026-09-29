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
about by name and version in OSV or Trivy, nor by an index with `--online`;
a submodule or a Soldeer git dependency at a full commit on a public forge
is asked about in OSV by that commit (REQ-FND-026).

## Rationale

Git submodules are a git mechanism, not a Solidity one, so their island is
named for git and other languages may share it. OSV has no Solidity
ecosystem; Soldeer's registry API says nothing about dependencies
(REQ-SOLIDITY-011).

## Acceptance criteria

1. The plugin declares exactly these islands, none a standard library.
