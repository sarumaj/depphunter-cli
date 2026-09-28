---
id: REQ-CMAKE-010
title: CMake files read without configuring
scope: cmake
type: limitation
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall not** run CMake. Conditions (`if()`), loops
(`foreach()`), function calls and `string()` operations are not evaluated:
every branch counts, a variable takes the last value set in the file, and a
value computed at configure time (`find_package` results,
`<name>_SOURCE_DIR` of fetched content, the binary directories, variables
set inside an included file or by a parent with `PARENT_SCOPE`) is unknown.
`target_link_libraries()` and `target_include_directories()` are not
read, so the sources' includes of fetched content are matched to it by name
(REQ-CPP-017), not by the include directories a target adds.

## Rationale

The vendored tree-sitter cmake grammar was measured on shallow clones of
fmtlib/fmt, nlohmann/json, google/benchmark, catchorg/Catch2 and
Farama-Foundation/Arcade-Learning-Environment: 111 files, no ERROR nodes,
but 6.4 ms per file (47 ms for one Catch2 file). The scanner reads the same
files in well under a millisecond each (the plugin's whole analysis of
nlohmann/json's 46 CMake files, resolution included, took 55 ms), and
nothing the plugin needs is below the level of commands and arguments.

## Acceptance criteria

1. A `set()` inside `if()` counts; `add_subdirectory(${json_SOURCE_DIR})`
   after `FetchContent_Populate(json)` and
   `include(${CMAKE_CURRENT_LIST_DIR}/@targets@.cmake)` are dropped.
