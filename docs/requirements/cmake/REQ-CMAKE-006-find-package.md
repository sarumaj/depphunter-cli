---
id: REQ-CMAKE-006
title: find_package attributed like the includes
scope: cmake
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`find_package(Name)` and `find_dependency(Name)` **shall** resolve to the
`CMakeLists.txt` of a project of that name in the repository (other than the
file's own); else to the package a vcpkg or Conan manifest over the file
declares, or else content the project fetches, matched as the cpp plugin matches
the library's headers (REQ-CPP-012, REQ-CPP-017) - `ZLIB` as `<zlib.h>`,
`nlohmann_json` as `<nlohmann/json.hpp>`, `Eigen3` as `<Eigen/...>`, `GTest` as
`<gtest/...>`, each Boost and Qt component on its own (`Boost filesystem` as
`<boost/filesystem/...>`, `Qt6 Widgets` as `<QtWidgets/...>`, also for
`Qt${QT_VERSION_MAJOR}`), any other name as `<name/...>` in lower case; else to
the project's own `FindName.cmake` on `CMAKE_MODULE_PATH`; else to content the
project fetches under that name (REQ-CMAKE-007); else, for a tool or platform
package CMake's find modules find (Threads, OpenMP, PkgConfig, Python, Java,
Doxygen, Git, CUDA, MPI, OpenGL, X11 and the like), to the `cmake-std` module
`FindName`; else to the unresolved `c-external` library the cpp plugin names for
those headers.

## Rationale

The build file and the sources including the library's headers must meet on
one package node, whichever manifest - or none - declares it.

## Acceptance criteria

1. With `vcpkg.json` declaring `fmt`, `zlib` and `boost-filesystem`,
   `find_package(fmt)`, `find_package(ZLIB)` and `find_package(Boost ...
   filesystem ...)` are those vcpkg packages; Boost's `system` component is
   the `c-external` library `boost`.
2. `find_package(nlohmann_json)`, `find_package(GTest)`,
   `find_package(Qt6 COMPONENTS Widgets)` and `find_package(Boost ...
   system)` land on the same targets as `#include <nlohmann/json.hpp>`,
   `<gtest/gtest.h>`, `<QtWidgets/QWidget>` and
   `<boost/system/error_code.hpp>` in a source file.
3. `find_package(Threads)` is `cmake-std` `FindThreads`;
   `find_package(Shop)` in `tests/` is the root `CMakeLists.txt`.
