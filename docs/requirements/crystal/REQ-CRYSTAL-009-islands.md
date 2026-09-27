---
id: REQ-CRYSTAL-009
title: The shards and standard library islands
scope: crystal
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

Shards **shall** form the `shards` island ("Crystal shards") and the
standard library the hidden `crystal-std` island ("Crystal standard
library"); `shards:` **shall** be a private-pattern prefix. Shards
**shall not** be asked about in OSV, which has no Crystal ecosystem, nor
mapped from Trivy, which has no Crystal package type, and `--online` has no
index to ask (shards are git repositories; shardbox.org offers no
dependency API). crystalline serves `--lsp` references for `.cr` files
(REQ-LSP-002).

## Rationale

Nothing else in the tool knows shards; they need their own island.

## Acceptance criteria

1. The plugin declares `shards` and the Std `crystal-std`, and every
   package target it returns is in one of them.
