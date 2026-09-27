---
id: REQ-FORTRAN-006
title: Included files
scope: fortran
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`include 'file'`, `#include "file"` and `#:include "file"` **shall**
resolve to the project file relative to the including file, then under the
include directories (`[library] include-dir`, `include` by default) of the
fpm projects over it, then under an `include/` directory or in the directory
itself of each ancestor directory. A file of a library's installation
(`mpif.h`, `fftw3.f03`, `netcdf.inc`) and a system header
(`#include <file>`) not in the project **shall** be the library's
(REQ-FORTRAN-007); any other missing file **shall** be dropped.

## Rationale

Include files carry common blocks and parameters shared between files,
the only way fixed-form code shares them.

## Acceptance criteria

1. `include 'shop.inc'` resolves to `src/shop.inc`, `include
   'common.inc'` to `include/common.inc` and `INCLUDE 'params.inc'` to
   `legacy/params.inc`.
2. `include 'mpif.h'` is the C library `mpi`; `include 'nowhere.inc'` is
   dropped.
