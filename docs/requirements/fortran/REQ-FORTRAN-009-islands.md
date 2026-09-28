---
id: REQ-FORTRAN-009
title: Islands
scope: fortran
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Fortran plugin **shall** declare three islands: **fpm packages**
(`fpm`, a private-pattern prefix), **Fortran external modules**
(`fortran-external`, a private-pattern prefix: modules neither the project
nor a known package provides) and the hidden **Fortran intrinsic modules**
(`fortran-std`, Std), plus the cpp plugin's `vcpkg`, `conan` and
`c-external` islands for C libraries. OSV and Trivy have no Fortran or fpm
ecosystem, and fpm's registry has no documented dependency API, so fpm
packages are not asked about by `--online` and are checked for advisories
only by their commit, when one pins them to a git repository on a public
forge (REQ-FND-026).

## Rationale

Every other plugin names its package manager's island and its standard
library the same way; MPI and HDF5 are C libraries whichever language
uses them.

## Acceptance criteria

1. Every resolved import of both fixtures is local or of a declared
   island.
