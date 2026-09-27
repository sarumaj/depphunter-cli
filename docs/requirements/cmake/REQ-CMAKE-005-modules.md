---
id: REQ-CMAKE-005
title: Modules on CMAKE_MODULE_PATH and CMake's own
scope: cmake
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`include(Name)` **shall** resolve to `Name.cmake` in a directory on
`CMAKE_MODULE_PATH` as the file and the `CMakeLists.txt` files above it set
it (then as any project file sets it), else to a module CMake ships (the
hidden `cmake-std` island: FetchContent, ExternalProject, GNUInstallDirs,
CTest, GoogleTest, CMakePackageConfigHelpers, the Check* modules and the
others, and any `Find*` module), else to the one project file named
`Name.cmake` (the nearest when several are), else be dropped.

## Rationale

A project's own modules are the ones it maintains; CMake's are part of the
tool, like a standard library.

## Acceptance criteria

1. `include(Warnings)` with `cmake/` on `CMAKE_MODULE_PATH` imports
   `cmake/Warnings.cmake`, from the root and from `src/`.
2. `include(FetchContent)` and `include(CTest)` are `cmake-std` packages.
3. `include(NotAnywhere)` is dropped.
