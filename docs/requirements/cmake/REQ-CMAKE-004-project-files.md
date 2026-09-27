---
id: REQ-CMAKE-004
uuid: 92f0e902-9604-4ebe-acaf-9265eb03582b
title: Subdirectories, included files and sources
scope: cmake
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`add_subdirectory(dir)` **shall** be an import of `dir/CMakeLists.txt`;
`include(path)` with a path (a `/` or a `.cmake` ending) of that file; the
source files of `add_library()`, `add_executable()` (and the Qt, pybind11,
nanobind and CUDA variants), `target_sources()` and the `SOURCES` of
`add_custom_target()` of those files; and `configure_file()`'s input of its
template. Relative paths are relative to the current source directory: a
`CMakeLists.txt`'s own directory, or for another file (whose includer is
unknown) its directory and then each one above. Keywords (`STATIC`,
`PRIVATE`, `FILE_SET` and its name, `BASE_DIRS`), values without an
extension and generator expressions **shall not** be sources; a path naming
no project file **shall** be dropped.

## Rationale

These edges tie the build to the code it compiles and to the build files it
reads: a map of a C or C++ project otherwise shows its build files as
islands.

## Acceptance criteria

1. `add_subdirectory(src)` imports `src/CMakeLists.txt`,
   `add_subdirectory(missing)` is dropped.
2. `add_executable(shop main.cpp $<...> generated.cpp)` imports
   `src/main.cpp`, records no import for the generator expression and drops
   `generated.cpp`.
3. `target_sources(shop PRIVATE FILE_SET HEADERS BASE_DIRS dir FILES a.hpp)`
   imports only `a.hpp`.
