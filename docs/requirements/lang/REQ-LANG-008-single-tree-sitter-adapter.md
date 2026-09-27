---
id: REQ-LANG-008
title: Single tree-sitter adapter
scope: lang
type: constraint
priority: must
status: implemented
verification:
  - inspection
---

## Statement

Plugins **shall** reach the tree-sitter runtime only through the package
`internal/lang/treesitter`.

## Rationale

Keeping the runtime behind one adapter allows it to be replaced, for example by
a WASM/`wazero` build, in one place.

## Acceptance criteria

1. Only `internal/lang/treesitter` imports the runtime package
   `github.com/odvcencio/gotreesitter` itself; plugins import only its grammar
   packages.
