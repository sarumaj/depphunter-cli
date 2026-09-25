---
id: REQ-LANG-013
uuid: 0dcea9ff-acab-43b1-9794-aa685b28de65
title: Tree-sitter runtime version pinned
scope: lang
type: constraint
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The build **shall** pin the gotreesitter module to one exact version.

## Rationale

The library is young and changes quickly; a pinned version keeps parse results
reproducible, and updates arrive as reviewed pull requests.

## Acceptance criteria

1. `go.mod` requires `github.com/odvcencio/gotreesitter` at an exact version and
   `go.sum` records its checksum.
