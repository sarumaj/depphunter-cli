---
id: REQ-CMAKE-003
uuid: 1a7e289e-ba90-4349-a387-dc0b9577a615
title: Variables evaluated
scope: cmake
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Paths, names and URLs **shall** be evaluated from literals, the variables
the file sets before their use (`set()`, `list(APPEND|PREPEND)`,
`get_filename_component(... DIRECTORY|ABSOLUTE)`, `project()`), nested
references (`${${P}_DIR}`), the variables the `CMakeLists.txt` files of the
directories above set (a value referring to its own name reads the one
above), and CMake's directory variables: `CMAKE_CURRENT_SOURCE_DIR`,
`CMAKE_CURRENT_LIST_DIR`, `CMAKE_SOURCE_DIR` (the topmost directory with a
`CMakeLists.txt`), `PROJECT_SOURCE_DIR` and `<Project>_SOURCE_DIR` (the
directory whose `CMakeLists.txt` declares the project) and `PROJECT_NAME`.
Variables set inside a `function()` or `macro()` body **shall not** leak
out of it. A value holding an unknown variable, an environment variable, a
generator expression, a binary-directory variable or a `@VAR@` placeholder
**shall not** be followed.

## Rationale

Real builds compute their paths from these variables; `add_subdirectory()`
nests directory scopes, and the layout says which directory a
`CMakeLists.txt` is added from in practice.

## Acceptance criteria

1. `${SHOP_INCLUDE}/shop/core.hpp` in `src/` reads `SHOP_INCLUDE` from the
   root `CMakeLists.txt`, `${Shop_SOURCE_DIR}/include/shop/api.hpp` and
   `${PROJECT_SOURCE_DIR}/test_cart.cpp` in a sub-project resolve.
2. `${ROOT}/src/util.cpp` after `get_filename_component(ROOT
   ${CMAKE_CURRENT_SOURCE_DIR}/.. ABSOLUTE)` resolves to `src/util.cpp`.
3. `${CMAKE_CURRENT_BINARY_DIR}/config.h` and `@targets@.cmake` are not
   followed.
