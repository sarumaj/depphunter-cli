---
id: REQ-FORTRAN-010
uuid: d5752b90-7b4a-46a9-b952-1250e0f349fb
title: Read by a scanner, not the grammar
scope: fortran
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Fortran **shall** be read by a statement reader of its own, not a
tree-sitter grammar, and any input **shall** be read in time linear in its
size without a panic.

## Rationale

The vendored grammar reads free form only: 2099 of 2141 fixed-form LAPACK
files parsed with errors, and it took 47 ms per file on json-fortran (2.1 s
for one file).

## Acceptance criteria

1. Every prefix of every fixture file and 200 KB runs of each construct
   extract without a panic within the time bound.
