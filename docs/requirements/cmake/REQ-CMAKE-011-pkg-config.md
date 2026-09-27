---
id: REQ-CMAKE-011
uuid: fa6ed2f0-e562-4bb2-8a81-2a6485819053
title: pkg-config modules
scope: cmake
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Each module of `pkg_check_modules()` and `pkg_search_module()`
(`glib-2.0>=2.40`) **shall** resolve to the package a vcpkg or Conan
manifest declares under its name (without a `lib` prefix and a version
suffix), else to the module of the `pkg-config` island with the constraint
as its version; a constraint `=version` pins.

## Rationale

pkg-config names the system libraries a build links; they are packages of
the platform, not of a manifest.

## Acceptance criteria

1. `pkg_check_modules(GLIB REQUIRED IMPORTED_TARGET glib-2.0>=2.40 gio-2.0)`
   imports `pkg-config` `glib-2.0` (version `>=2.40`) and `gio-2.0`.
