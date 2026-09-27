---
id: REQ-CMAKE-008
uuid: 139ef5d0-4b77-4ecd-99e7-e3c79f2c4e7f
title: CMake symbols
scope: cmake
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record as symbols the projects a file declares
(`project`), its targets (`library`, `executable`, `alias`, and `target` for
`add_custom_target()`; not IMPORTED ones), its `function()` and `macro()`
definitions, its `option()`s and the variables it sets in the cache
(`cache`), each named as far as the file's variables tell.

## Rationale

Targets, functions and options are what other build files and users refer
to.

## Acceptance criteria

1. `add_library(shop_core STATIC ...)` is the library `shop_core`,
   `add_library(Shop::core ALIAS shop_core)` the alias `Shop::core`,
   `add_library(ext::zlib UNKNOWN IMPORTED)` nothing.
2. `function(shop_warnings target)` and `macro(shop_option name)` are a
   function and a macro.
