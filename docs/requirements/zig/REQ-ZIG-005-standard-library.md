---
id: REQ-ZIG-005
uuid: 3b2366ab-2c4c-4e93-8236-405d6dafb68a
title: Standard library
scope: zig
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`@import("std")` and `@import("builtin")` **shall** be packages `std` and
`builtin` of the hidden `zig-std` island (Zig standard library), and a
`build.zig.zon`'s `.minimum_zig_version` **shall** be the package `zig` of
that island with the version `>= x`.

## Rationale

The standard library ships with the compiler; hiding it keeps the map on the
project's own dependencies.

## Acceptance criteria

1. `@import("std")` is `zig-std` `std`; `.minimum_zig_version =
   "0.14.0"` is `zig` `>= 0.14.0`.
