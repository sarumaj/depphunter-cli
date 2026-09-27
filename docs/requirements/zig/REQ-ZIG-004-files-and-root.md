---
id: REQ-ZIG-004
uuid: ee30008e-4125-4156-b544-cf217781f58a
title: Files and the compilation root
scope: zig
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`@import("x.zig")` (or a `.zon`) and `@embedFile("x")` **shall** resolve to
the file relative to the importing file, `b.path("x")` and `.{ .path = "x"
}` to the file or directory relative to the build root (the nearest
directory above with `build.zig` or `build.zig.zon`), and be dropped when
there is no such file in the repository. `@import("root")` **shall** resolve
to the root source file of the compilation that reaches the file through
relative and module imports, taken from the compile steps (`addExecutable`,
`addTest`, `addLibrary` and the like) of the file's own package when any
reach it: the only one, else the only one that is not a test; otherwise, and
in the root file itself, it is dropped.

## Rationale

`@import("root")` is how library code reaches the program's overrides.

## Acceptance criteria

1. `@import("cli/args.zig")` is `src/cli/args.zig`, `@embedFile("assets/banner.txt")`
   is `src/assets/banner.txt`, `@import("missing.zig")` is dropped.
2. `@import("root")` in `src/shop.zig` is `src/main.zig`, which the
   executable's root module reaches besides the test root; in
   `tests/all.zig` it is dropped.
