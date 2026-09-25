---
id: REQ-MOD-010
uuid: f3c61d80-5fd6-4251-af88-c1797cb38fe2
title: Depends edge kind
scope: mod
type: interface
priority: must
status: implemented
verification:
  - unit
---

## Statement

An edge from a package node to a package node it depends on **shall** have the
kind `depends`, distinct from `import`.

## Rationale

Keeping package-to-package edges a kind of their own keeps the number of files
importing a package a count of files.

## Acceptance criteria

1. With `--resolve-depth` enabled, an edge between two package nodes has the
   kind `depends`.
2. An edge from a file node to a package node keeps the kind `import`.
3. The UI's count of a package's importers does not include `depends` edges.
