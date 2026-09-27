---
id: REQ-CPP-004
uuid: 78d03bfe-8933-455e-b635-2b2b25e34816
title: Includes resolved to project files
scope: cpp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The C/C++ plugin **shall** resolve an include to the project file found
first, in this order: for `#include "x"`, the includer's directory; the include
directories (`-I`, `-iquote`, `-isystem`, `-idirafter`, and `/I`,
`/external:I` of MSVC) of the includer's entry in a `compile_commands.json` at
the root or in a `build*/` or `cmake-build-*/` directory, or of every entry for
a file without one; `include/`, `src/` and the root; and last the one project
file whose path ends in `/x`, or when several do, the one sharing the most
leading directories with the includer, if no other shares as many.

## Rationale

This is the search order of the compilers, with the include path taken from
the compilation database that CMake, Meson, Bear and others write. Without one,
projects keep headers in the conventional directories or refer to them by
their path below an include directory.

## Acceptance criteria

1. With `-I../include` in `build/compile_commands.json`, `#include "app/app.hpp"`
   resolves to `include/app/app.hpp`.
2. The `command` form of an entry is read as well as `arguments`, and a
   relative `directory` is taken from the database's own directory.
3. Two files ending in `util/util.h` are told apart by the includer's `-I`.
4. An include directory outside the project and an absolute include are not
   followed.

## Notes

Compilation databases are usually ignored by git and so not among the scanned
files; they are read from disk.
