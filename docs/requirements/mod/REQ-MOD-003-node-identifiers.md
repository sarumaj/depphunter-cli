---
id: REQ-MOD-003
uuid: 1955e9e3-4d08-4962-82af-09e43f086a7a
title: Node identifiers
scope: mod
type: interface
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** give every node an `id` that is unique within the document
and derived from what the node stands for: `d:<path>` for a directory,
`f:<path>` for a file, `s:<file path>#<symbol name>` for a symbol,
`e:<ecosystem>` for an ecosystem and `p:<ecosystem>:<package>` for a package,
where paths are relative to the analyzed root and slash-separated, and the root
directory is `d:.`.

## Rationale

Identifiers derived from paths and names are stable across runs, so the UI can
keep expansion, selection and filters across a live update, and edges can name
their ends without a lookup table.

## Acceptance criteria

1. The root directory node has the id `d:.`.
2. A file `util/util.go` has the id `f:util/util.go`; the standard-library
   package `fmt` of the Go ecosystem has the id `p:go-std:fmt`.
3. Two analyses of an unchanged project produce the same set of ids.
