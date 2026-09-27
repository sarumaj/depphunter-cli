---
id: REQ-FORTRAN-011
uuid: 11e2ceee-85cf-47c1-9d26-963ac5ab8cdb
title: Read without the compiler or fpm
scope: fortran
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Fortran **shall** be read without running the compiler, the C
preprocessor, fypp or fpm: every branch of `#if` counts, macros are not
expanded, fypp loops and substitutions are not run (so `stdlib`'s generated
module names such as `stdlib_blas_constants_${k}$` are not known), fixed
form's insignificant blanks inside names (`SUB ROUTINE`) are not joined,
columns past 72 are read, a module of a package fpm has not fetched is
attributed by the curated table or the declared name it spells, and
fpm's registry is not asked (no `--online`).

## Rationale

Conditional compilation and templates are resolved only by a build; which
package provides a module only by fpm.

## Acceptance criteria

1. Both branches of an `#ifdef` are read: `use mpi` inside `#ifdef
   USE_MPI` is an import.
