---
id: REQ-FORTRAN-004
uuid: 69c8abb6-04ed-45f4-b8e6-c4db4a8ce0b0
title: Modules resolved to files
scope: fortran
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A used module **shall** resolve to the project file that defines it, found
through an index of the `module` and `submodule` statements of every Fortran
source of the project, compared case-insensitively, whatever build system
the project uses (fpm, CMake, Make). When several files define it, the one
sharing the longest directory prefix with the using file **shall** be
chosen; a module the using file defines itself **shall** be dropped. A
submodule's parent **shall** resolve to the parent submodule's file when
one is named, else the ancestor module's.

## Rationale

Most Fortran code is built by CMake or Make, where no manifest says where
a module lives; a module's name is all a `use` statement gives.

## Acceptance criteria

1. `use shop_cart` resolves to `src/cart.F90`, which declares
   `MODULE Shop_Cart`.
2. `submodule (shop_cart:cart_impl) cart_more` resolves to
   `src/cart_impl.f90`.
3. In the CMake fixture without `fpm.toml`, `use constants` resolves to
   `src/constants.f90`.
