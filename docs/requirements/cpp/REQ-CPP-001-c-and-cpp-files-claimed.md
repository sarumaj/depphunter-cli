---
id: REQ-CPP-001
uuid: a2a5e2e6-0371-4508-812d-9198cf71bb94
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
case), parsing `.c` files with the C grammar and every other one, `.h`
included, with the C++ grammar.

## Rationale

C and C++ files include each other, so one plugin with one resolver maps
both. A `.h` file may be either language and its extraction may depend only
on its content and extension; the C++ grammar reads nearly all C headers, while
the C grammar reads none of the classes, namespaces and templates of a C++
header. A `.c` file is C, where `new` or `class` is an ordinary name that the
C++ grammar would not read.

## Acceptance criteria

1. `src/main.cpp`, `lib/util.c`, `include/app/app.hpp` and a `.h` file are
   analyzed; `compile_commands.json` is not.
2. In a `.c` file, a function declaring a variable named `new` is still a
   symbol.

## Notes

A C header using a C++ keyword as a name may lose the definitions around it
(see REQ-CPP-008).
