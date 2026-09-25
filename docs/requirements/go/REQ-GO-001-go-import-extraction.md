---
id: REQ-GO-001
uuid: 4601fb57-a8fb-41c0-a641-03c72ccfea20
title: Go import extraction
scope: go
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Go plugin **shall** claim non-binary `.go` files, **shall** extract their
imports with the standard library's `go/parser`, **shall** keep the imports of
the partial syntax tree when a file has syntax errors, and **shall** ignore the
cgo pseudo-package `C`.

## Rationale

`go/parser` is exact for Go and ships with the toolchain; a file being edited
often does not compile, and a map should still show its imports.

## Acceptance criteria

1. Every import path of a Go file is reported with its line.
2. A Go file with a syntax error still reports the imports before the error.
3. `import "C"` produces no import.
