---
id: REQ-ZIG-010
title: C headers of @cImport
scope: zig
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

`@cInclude("x.h")` **shall** be resolved as the cpp plugin resolves
`#include <x.h>` (project headers by include path, conventional directories
and suffix, the C standard library and system headers, vcpkg and Conan
manifests, else `c-external`).

## Rationale

A C header a Zig file translates is the same dependency a C file including
it has, and lands on the same node.

## Acceptance criteria

1. `@cInclude("stdio.h")` is `c-std` `stdio.h`; `@cInclude("shop.h")`
   is `include/shop.h`.
