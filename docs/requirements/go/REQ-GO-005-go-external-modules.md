---
id: REQ-GO-005
uuid: e2473dab-8a80-4c9e-8add-5fb5b6c40fe0
title: Go external modules
scope: go
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M1
  - docs/REQUIREMENTS.md M2
verification:
  - unit
---

## Statement

The Go plugin **shall** resolve an import of an external package to a package
node of the longest module path required by the owning module's `go.mod`,
carrying that requirement's version, and **shall** mark an external import that
no `require` covers as unresolved, named by its import path.

## Rationale

A module, not each of its packages, is the unit that is versioned and
downloaded.

## Acceptance criteria

1. `github.com/spf13/cobra/doc` with `require github.com/spf13/cobra v1.8.0`
   resolves to the package `github.com/spf13/cobra` version `v1.8.0`.
2. An import of a module missing from `go.mod` resolves to a package marked
   unresolved.
