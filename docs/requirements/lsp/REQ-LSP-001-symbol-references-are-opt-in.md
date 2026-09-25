---
id: REQ-LSP-001
uuid: a80d7e3c-fc22-41a3-9820-0e28b5efc39c
title: Symbol references are opt-in
scope: lsp
type: functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The system **shall** start language servers only when `--lsp` is given.

## Rationale

Decision: references start external servers and take seconds to minutes
(gopls: 533 definitions in 7.5 s on this repository).

## Acceptance criteria

1. A run without `--lsp` starts no language server, and `/api/references`
   answers 204.
