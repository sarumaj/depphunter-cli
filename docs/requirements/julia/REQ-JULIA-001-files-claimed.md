---
id: REQ-JULIA-001
uuid: 8feb36e2-edef-46ce-952e-68766aca1080
title: Files claimed
scope: julia
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The julia plugin **shall** claim Julia sources (`.jl`) and Pkg's files:
`Project.toml` and `JuliaProject.toml`, `Manifest.toml`,
`JuliaManifest.toml` and the versioned `Manifest-v1.11.toml` and
`JuliaManifest-v1.11.toml`, and `Artifacts.toml` and
`JuliaArtifacts.toml`, whose artifact names are its symbols. Projects,
manifests and artifact files **shall** have cache classes of their own,
since they share `.toml` with other tools' files.

## Rationale

Pkg's project says which packages a package or environment uses, its
manifest which versions they resolved to; an artifact is a binary download
no registry describes, so it is named but not linked.

## Acceptance criteria

1. The fixture's sources, projects (package, test, docs, tools), manifests
   (plain and versioned) and `Artifacts.toml` are analyzed; `Cargo.toml`,
   `pyproject.toml`, `project.toml`, `Manifest-vx.toml` and a binary
   `.jl` are not.
2. `Artifacts.toml`'s symbols are its artifacts `libshop` and `fonts`.
