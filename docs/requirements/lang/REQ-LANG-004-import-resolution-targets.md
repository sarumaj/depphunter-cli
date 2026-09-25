---
id: REQ-LANG-004
uuid: 935d03c1-0336-49dd-8b92-7394b3ca9ed7
title: Import resolution targets
scope: lang
type: interface
priority: must
status: implemented
verification:
  - unit
---

## Statement

A plugin **shall** resolve each extracted import to exactly one of: a project
path (a file or a directory), a standard-library package, or an external package
with its ecosystem and, where the ecosystem's manifests and lock files state it,
its version. An import that resolves to none of these (for example one pointing
outside the project) **shall** be dropped.

## Rationale

The three kinds of target are the three places a dependency can live on the map:
the mainland, a standard-library island and a package island.

## Acceptance criteria

1. A Go import of a package of the project resolves to its directory.
2. A Go import of `fmt` resolves to the standard-library package `fmt`.
3. A Go import of a required module resolves to that module with the version
   from `go.mod`.
4. A relative import that leaves the project produces no edge.
