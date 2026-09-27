---
id: REQ-MOD-008
title: Edge target kinds
scope: mod
type: interface
priority: must
status: implemented
verification:
  - unit
---

## Statement

The target of an import edge **shall** be a file node, a directory node (for
example a Go package) or a package node.

## Rationale

Import systems differ in what an import names: a JavaScript import names a file,
a Go import a directory, and an external import a package.

## Acceptance criteria

1. A relative JavaScript import produces an edge to a file node.
2. A Go import of a package inside the project produces an edge to a directory
   node.
3. An import of an external or standard-library package produces an edge to a
   package node.
4. An import resolving to a path that is neither a listed file nor a directory
   of the document produces no edge.
