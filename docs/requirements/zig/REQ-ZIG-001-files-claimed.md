---
id: REQ-ZIG-001
title: Files claimed
scope: zig
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Zig plugin **shall** claim Zig sources (`.zig`), the package manifest
`build.zig.zon` and other `.zon` files, and **shall not** claim files under
Zig's build output and caches (`zig-out`, `.zig-cache`, `zig-cache`,
`zig-pkg`). A `.zon` file other than `build.zig.zon` is data a program
imports: it is claimed as a file but **shall** declare no imports or
symbols, and its extraction **shall** be cached apart from a manifest's.

## Rationale

Zig's own caches hold generated translations of C headers and fetched
packages that are not the repository's code.

## Acceptance criteria

1. `src/main.zig`, `build.zig`, `build.zig.zon` and `data/config.zon` are
   claimed; `.zig-cache/o/1/cimport.zig`, `zig-out/lib/x.zig` and
   `zig-pkg/x-0.1.0-AAAA/build.zig.zon` are not.
2. `build.zig.zon` and `data/config.zon` have different cache classes.
