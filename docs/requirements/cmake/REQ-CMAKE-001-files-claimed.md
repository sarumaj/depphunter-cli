---
id: REQ-CMAKE-001
uuid: e90ca03d-8b53-4479-9e19-0e474db52e46
title: CMake files claimed
scope: cmake
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The `cmake` plugin **shall** analyze files named `CMakeLists.txt`, files
ending in `.cmake` or `.cmake.in` (package configuration templates) and the
presets files `CMakePresets.json` and `CMakeUserPresets.json`, and no other
`.txt` or `.json` file.

## Rationale

A CMake build is spread over one `CMakeLists.txt` per directory and the
modules and templates it includes; the presets name its toolchains.

## Acceptance criteria

1. `CMakeLists.txt`, `cmake/Deps.cmake`, `cmake/ShopConfig.cmake.in` and
   `CMakePresets.json` are analyzed; `presets/base.json` and `README.md` are
   not.
