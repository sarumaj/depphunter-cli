---
id: REQ-CMAKE-009
title: Presets files
scope: cmake
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

In `CMakePresets.json` and `CMakeUserPresets.json`, every configure, build,
test, package and workflow preset **shall** be a symbol of kind `preset`;
the files of `include` (relative to the presets file) and a
`toolchainFile` written relative to it or to `${sourceDir}` **shall** be
imports of those project files. A toolchain under `$env{...}` or an
absolute path **shall not** be recorded.

## Rationale

Presets pick the toolchain file a build uses and include each other.

## Acceptance criteria

1. `"include": ["presets/base.json"]` imports `presets/base.json`;
   `"toolchainFile": "${sourceDir}/cmake/toolchain.cmake"` imports
   `cmake/toolchain.cmake`; `$env{VCPKG_ROOT}/...` records nothing.
