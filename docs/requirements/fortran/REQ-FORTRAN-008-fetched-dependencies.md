---
id: REQ-FORTRAN-008
uuid: e400cc2a-7081-4ebd-badb-9a7cd22efbef
title: Dependencies fpm fetched
scope: fortran
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** read `build/cache.toml` and the dependencies fpm
fetched into `build/dependencies/<name>/` beside each `fpm.toml`: the
modules their sources define (for REQ-FORTRAN-007) and their own
`fpm.toml`, whose dependencies (not dev-dependencies or path dependencies)
**shall** answer `--resolve-depth` for that package, versioned by the
fetching project's cache and manifest, and reported as installed.

## Rationale

fpm has no lock file; what it fetched is the only offline record of a
dependency's own dependencies.

## Acceptance criteria

1. `fancy`'s dependencies are `json-fortran` (as the project declares it)
   and `leftpad` at the revision cache.toml records; `fancy` is installed
   and `plotter` is not.
