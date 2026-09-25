---
id: REQ-MOD-002
uuid: 1ba83628-1874-44aa-bbf0-bb99b9d1d8d4
title: Node kinds
scope: mod
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §3
verification:
  - unit
  - inspection
---

## Statement

Every node of the graph document **shall** have a `kind` that is one of `dir`,
`file`, `symbol`, `ecosystem` and `package`.

## Rationale

The UI lays out and aggregates by kind: directories and files form the mainland,
symbols the inside of an expanded file, ecosystems the islands and packages the
buildings on them.

## Acceptance criteria

1. An analysis of a project with a Go file that imports an external module
   produces nodes of all five kinds.
2. No node carries a kind outside the five listed.
