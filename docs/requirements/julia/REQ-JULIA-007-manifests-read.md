---
id: REQ-JULIA-007
uuid: 189a13e7-0439-4c1c-b937-455d223a06e1
title: Manifests read
scope: julia
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read manifests of format 2.0 (`[[deps.Name]]`) and
1.0 (top-level `[[Name]]`): each entry's `uuid`, `version`,
`git-tree-sha1`, `path`, `repo-url`, `repo-rev` and `deps` (a list, or
a table of names and UUIDs). The manifest resolving a project is the one in
its directory - found on disk when not committed, preferring a versioned
`Manifest-vX.Y.toml` (the newest) over a plain one and `JuliaManifest`
over `Manifest` - else the nearest above it. Every entry of a claimed
manifest **shall** be an import of it.

## Rationale

Manifests are often git-ignored in packages and committed in applications
and documentation environments.

## Acceptance criteria

1. The fixture's manifest gives nine imports: CairoPlot (added by URL,
   with its origin), three standard libraries, four pinned packages and
   Utils (developed, `src/Utils.jl`).
2. A format 1.0 manifest and a table-valued `deps` read; an entry is
   matched by UUID when one is given.
3. `tools/clean.jl` gets DataFrames 1.7.0 from `Manifest-v1.11.toml`, not
   1.6.0 from `Manifest.toml`.
