---
id: REQ-MOD-006
uuid: 7d5fb51c-31a6-4540-a2b6-21833565b6b3
title: Import edges
scope: mod
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §3
verification:
  - unit
---

## Statement

An edge **shall** carry `from` and `to` (node ids), `kind` and, where known,
`line` (the 1-based line of the import in the source file). An edge from a file
to what it imports **shall** have the kind `import`, and at most one edge
**shall** exist per ordered pair of nodes.

## Rationale

The import relation is the dependency structure the map exists to show; one edge
per pair keeps counts meaningful when a file imports several sub-packages of one
module.

## Acceptance criteria

1. A file importing an external package yields exactly one edge of kind `import`
   from the file node to the package node.
2. A second import of the same package from the same file adds no second edge.
