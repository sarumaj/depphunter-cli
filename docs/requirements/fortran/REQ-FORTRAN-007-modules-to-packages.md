---
id: REQ-FORTRAN-007
title: Modules resolved to packages
scope: fortran
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A module no project file defines **shall** resolve, in order: to the
hidden `fortran-std` island for intrinsic modules (`iso_fortran_env`,
`iso_c_binding`, `ieee_arithmetic`, `ieee_exceptions`, `ieee_features`, any
module used with `use, intrinsic`, and `omp_lib`/`omp_lib_kinds` as
`openmp`, `openacc` as `openacc`); to the C library a library's Fortran
module stands for (`mpi` and `mpi_f08` as `mpi`, `hdf5`, `netcdf`, PETSc,
FFTW, MKL, CUDA Fortran), through the cpp plugin's attribution so that `use
mpi` and `#include <mpi.h>` meet on one node (REQ-CPP-006); to the fpm
dependency fpm fetched into `build/dependencies/` that defines it; to a
declared dependency whose name the module spells (`json_module` for
json-fortran, `tomlf` for toml-f, `stdlib_kinds` for stdlib, `testdrive`
for test-drive); to a module `[build] external-modules` lists (the
`fortran-external` island); to the package a curated table names
(unresolved when no manifest declares it); to nothing for the project's own
missing module; else to an unresolved module of the `fortran-external`
island named by the module.

## Rationale

A Fortran module name does not say which package defines it; fpm's
fetched sources, the manifest and a small table of well-known packages
do.

## Acceptance criteria

1. `use iso_c_binding` and `use omp_lib` are `fortran-std`; `use mpi_f08`
   and `use netcdf` are `c-external` `mpi` and `netcdf`.
2. `use fancy_core` is `fancy` (defined under `build/dependencies/fancy`),
   `use plotter_axes` is `plotter`, `use nothere_mod` an unresolved
   `fortran-external` module and `use shop_missing` is dropped.
3. In the CMake fixture, `use stdlib_math` is an unresolved fpm package
   `stdlib` and `use unknown_mod` an unresolved `fortran-external`
   module.
