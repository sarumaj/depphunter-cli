---
id: REQ-GO-004
uuid: effdc9f3-2ef1-47d1-a0d6-0e70a58a1d1a
title: Go standard-library packages
scope: go
type: functional
priority: must
status: implemented
verification:
  - unit
  - e2e
---

## Statement

The Go plugin **shall** resolve an import whose first path element contains no
dot, and which is not a package of the project, to a package node of that import
path in the ecosystem `go-std` ("Go standard library"), declared as a standard
library.

## Rationale

Standard-library dependencies are part of what a package draws in; declaring the
ecosystem a standard library lets the UI hide it by default.

## Acceptance criteria

1. `fmt` and `net/http` resolve to `p:go-std:fmt` and `p:go-std:net/http`.
2. Analyzing this repository, `internal/lang/golang` has an edge to the package
   `go/parser` on the Go standard-library island, and its dependents are listed.
