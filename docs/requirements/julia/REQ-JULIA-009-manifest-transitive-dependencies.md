---
id: REQ-JULIA-009
title: Manifests answer --resolve-depth
scope: julia
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** answer what a Julia package depends on from the
`deps` of the manifest entry that pinned it (matched by version or tree
hash): standard libraries as `julia-std`, the others as the same manifest
resolves them; developed packages are left out.

## Rationale

A manifest records the whole resolved graph, so no registry has to be asked.

## Acceptance criteria

1. JSON 0.21.4 depends on Dates, Mmap and Unicode (standard libraries) and
   Parsers 2.8.1 (pinned); JSON 0.20.0 and a standard library get nothing.
