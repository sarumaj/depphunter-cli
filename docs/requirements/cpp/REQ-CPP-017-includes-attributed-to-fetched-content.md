---
id: REQ-CPP-017
title: Includes attributed to fetched content
scope: cpp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An include that no vcpkg or Conan package claims (REQ-CPP-012) and that
would go to `c-external` **shall** resolve to the content the project's CMake
files fetch (`FetchContent_Declare()`, `ExternalProject_Add()`, CPM.cmake,
REQ-CMAKE-007) whose name - the name it is declared under, its repository's
name or `owner-repository`, folded like the include's - is one of the
include's candidate names (its first directory or bare header name, a known
alias, `lib` before it). The declaration in the including file's directory or
the nearest one above it **shall** win, else the shallowest one in the
project. Content that is a project file is not fetched and **shall not** be
matched. `find_package()` **shall** land on the same package
(REQ-CMAKE-006).

## Rationale

A project that fetches its dependencies with CMake declares them nowhere else:
`#include <doctest/doctest.h>` beside
`CPMAddPackage(NAME doctest GIT_REPOSITORY https://github.com/doctest/doctest ...)`
is that package, not an unknown library named `doctest`.

## Acceptance criteria

1. `<doctest/doctest.h>`, `<magic_enum.hpp>`, `<catch2/...>`, `<gtest/...>`
   and `<nlohmann/json.hpp>` resolve to the fetched packages the fixture's
   `cmake/Deps.cmake` declares, equal to the build file's targets;
   `<cxxopts.hpp>` stays `c-external`, and so do all of them without the
   cmake plugin's reader.
2. `FetchContent_Declare(ext_opts GIT_REPOSITORY .../jarro2783/cxxopts)` is
   found by `<cxxopts.hpp>`; a tarball of the repository is not matched.
3. A declaration in the source's directory beats one in the root, which beats
   one in a sibling directory; a package a manifest declares comes first.
