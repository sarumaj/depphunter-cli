---
id: REQ-EXT-005
uuid: e36ec176-0827-44c0-bc30-ca2aec17dad0
title: Repeated node on a branch shown once and closed
scope: ext
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M15
  - README.md The panel beside the code
verification:
  - extension
---

## Statement

When a branch of the Dependencies view reaches a node that is already open
further up the same branch, the tree **shall** show that node once more, marked
`↻`, as a leaf that cannot be expanded.

## Rationale

Dependency graphs contain cycles, and a tree that followed one would not end.

## Acceptance criteria

1. A package that depends on something that depends back on it can be opened
   down to its repeat and no further.
2. The repeated row's description contains `↻ already above` and it has no
   children.
