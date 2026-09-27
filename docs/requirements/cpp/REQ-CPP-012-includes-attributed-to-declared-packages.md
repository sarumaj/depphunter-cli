---
id: REQ-CPP-012
title: Includes attributed to declared packages
scope: cpp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An include that would go to `c-external` **shall** resolve to the vcpkg or
Conan package declared in the manifests nearest the including file (its
directory, then each one above it) whose name, ignoring case and `-` against
`_`, is the include's library name, a known alias of it, or the name with
`lib` before it; for `<boost/x/...>` the port `boost-x` comes before
`boost`, and for a Qt module's directory (`<QtCore/...>`) its own port
before `qtbase`, `qt5-base`, `qt6`, `qt5` and `qt`. vcpkg is tried
before Conan in one directory. A file with no manifest in its directory or
above **shall** take the package from the project's other manifests, the
shallowest directory first, then by name. An include no declared package matches
**shall** stay in `c-external`.

## Rationale

A library's headers usually sit in a directory named after it, but a port or
recipe is named by its packager: `<nlohmann/json.hpp>` is `nlohmann-json`,
`<curl/curl.h>` is Conan's `libcurl`, `<Eigen/Dense>` is `eigen3`.

## Acceptance criteria

1. `<nlohmann/json.hpp>` with vcpkg `nlohmann-json` declared resolves to
   it; with Conan `nlohmann_json` declared, to that.
2. `<boost/asio.hpp>` resolves to `boost-asio`, and
   `<boost/filesystem.hpp>` stays `c-external` `boost` when neither
   `boost-filesystem` nor `boost` is declared.
3. A manifest in a subdirectory applies to its files, and a package it lacks
   is taken from a manifest above it, never from a sibling's.
4. A file under no manifest resolves `<spdlog/spdlog.h>` to the Conan
   `spdlog` a sibling directory's conanfile declares.

## Notes

The alias table (internal/lang/cpp/packages.go `headerAliases`) covers
well-known cases only, such as `gtest`/`googletest`, `Eigen`/`eigen3`,
`SDL2`, `GLFW`/`glfw3`, `google`/`protobuf` and `absl`/`abseil`.
