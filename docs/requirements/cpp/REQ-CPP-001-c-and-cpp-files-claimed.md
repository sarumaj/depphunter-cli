---
id: REQ-CPP-001
title: C and C++ files claimed
scope: cpp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The C/C++ plugin **shall** analyze files ending in `.c`, `.h`, `.cc`, `.cpp`,
`.cxx`, `.c++`, `.hpp`, `.hh`, `.hxx`, `.h++`, `.ipp` and `.inl` (in any
case), reading `.c` files as C and every other one, `.h` included, as C++,
except a `.h` file the scan labels Objective-C (REQ-OBJC-001).

## Rationale

C and C++ files include each other, so one plugin with one resolver maps
both. A `.h` file may be either language and its extraction may depend only
on its content and extension; reading C++ covers nearly all C headers, while
reading C would miss the classes, namespaces and templates of a C++ header. A
`.c` file is C, where `new` or `class` is an ordinary name that C++ would not
read.

## Acceptance criteria

1. `src/main.cpp`, `lib/util.c`, `include/app/app.hpp` and a `.h` file are
   analyzed; `compile_commands.json` is not, nor an Objective-C header.
2. In a `.c` file, a function declaring a variable named `new` is still a
   symbol.

## Notes

A C header using a C++ keyword as a name may lose the definitions around it
(see REQ-CPP-008).
