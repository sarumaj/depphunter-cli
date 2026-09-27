---
id: REQ-JULIA-005
uuid: 7ed55c81-f4f8-4eda-96bc-f2fd7ffbf6f9
title: Standard library
scope: julia
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`Base`, `Core` and Julia's standard libraries (LinearAlgebra, Dates, Test,
Pkg, ... and the JLL libraries Julia links) **shall** resolve to the hidden
`julia-std` island, unless the governing manifest's entry for the name has a
`git-tree-sha1` - an upgradable standard library (Statistics,
SparseArrays) installed from a registry, which is a Julia package pinned to
its version.

## Rationale

Standard libraries are listed in `[deps]` with their UUIDs like any
package, but they ship with Julia; a manifest entry without a tree hash is
one Julia provides.

## Acceptance criteria

1. In the fixture, LinearAlgebra, Test, Dates and Mmap are `julia-std`;
   Statistics, whose manifest entry has a tree hash, is Julia 1.11.1
   (pinned).
