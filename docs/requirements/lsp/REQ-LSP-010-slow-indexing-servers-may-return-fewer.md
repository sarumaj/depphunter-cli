---
id: REQ-LSP-010
uuid: 5c2f6391-42b5-4eea-a56c-60e7ed1d5ef0
title: Slow-indexing servers may return fewer references
scope: lsp
type: limitation
priority: must
status: implemented
verification:
  - manual
---

## Statement

Language servers that index slowly (rust-analyzer, jdtls) **may** answer before
indexing finishes and so return fewer references within the time budget.

## Rationale

The system does not wait for a server-specific "indexing finished" signal.

## Acceptance criteria

1. A first `--lsp` run on a Rust project that rust-analyzer has not yet indexed
   may yield fewer reference edges than a later run; the edges it did find are
   still served.

## Notes

The README lists the supported servers but does not state this limitation.
