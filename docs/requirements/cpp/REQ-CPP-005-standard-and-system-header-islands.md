---
id: REQ-CPP-005
uuid: 6b268883-c65c-4667-b096-ff4f5b1a1827
title: Standard and system header islands
scope: cpp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The C/C++ plugin **shall** assign an include the project does not resolve to
`c-std` when it is a C standard header (`stdio.h`), to `cpp-std` when it is a
C++ standard header (`vector`, `cstdio`, `experimental/*`), and to `c-system`
when it is a header of the operating system or the compiler (POSIX,
`sys/*`, Linux, Apple frameworks, Windows, intrinsics), each named as written
and each a standard-library island. A file of the project **shall not** be
taken for a standard or system header included with `<>` except through an
include directory of the compilation database.

## Rationale

Standard and platform headers are no dependency to manage and are hidden
unless standard libraries are shown. A project's `tests/stdio.h` is not what
`<stdio.h>` means without an include path saying so.

## Acceptance criteria

1. `<vector>` resolves to `cpp-std`, `<stdio.h>` to `c-std` and
   `<sys/socket.h>` to `c-system`.
2. `<stdio.h>` does not resolve to a project file `tests/stdio.h`.
