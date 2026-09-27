---
id: REQ-GO-003
title: Multi-module local resolution
scope: go
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Go plugin **shall** read every `go.mod` of the project and **shall** resolve
an import whose path lies within any of these modules' paths to the project
directory holding that package.

## Rationale

Repositories frequently contain several modules (tools, examples, sub-modules)
that import each other.

## Acceptance criteria

1. In a repository with modules `app` and `lib`, `example.com/app/internal/util`
   resolves to `app/internal/util`.
2. An import of `example.com/lib/sub` from module `app` resolves to `lib/sub`.
3. An unreadable or invalid `go.mod` is skipped without failing the analysis.

## Notes

Each file belongs to the module of the deepest `go.mod` above it. `replace`
directives are honored as well.
