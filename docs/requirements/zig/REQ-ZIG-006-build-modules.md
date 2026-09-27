---
id: REQ-ZIG-006
uuid: 1d496449-a4ce-4be3-b49a-274b57429980
title: Modules wired by build.zig
scope: zig
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`@import("name")` of a module **shall** resolve to what the build code of
the file's package (build.zig and any file of the package using the build
API) wires under that name: `.imports = &.{ .{ .name = "n", .module = m } }`
(and Zig 0.11's `.dependencies`), `addImport("n", m)`, `addAnonymousImport`,
0.11's `step.addModule("n", m)`, with `m` followed through variables, `if
(...) |capture|`, struct fields and function returns to a module made by
`b.createModule` or `b.addModule` from a `root_source_file` (a project
file), a `build.zig.zon` dependency's `.module("x")` (the package, or for a
`.path` dependency the root file of the module that package's build.zig
exports as `x`) or a generated module (`addOptions`, `createModule()` of
options or a generated file), which **shall** be dropped. Unwired, the name
**shall** resolve to a module the package's build code exports with
`b.addModule`, else the package's dependency of that name (`-` and `_`
alike), else a module exactly one other package of the repository exports;
else it is dropped. In `build.zig` itself a dependency's name **shall** be
that dependency's `build.zig` (a `.path` one) or its package.

## Rationale

A Zig module name means nothing without the build graph, and nearly every
project wires its dependencies' modules under their own names.

## Acceptance criteria

1. In the fixture, `shop` (`.imports`), `known-folders`
   (`b.dependency(...).module(...)`), `vaxis` (a variable's module),
   `utils` (a path dependency's exported module), `model` (a function's
   return), `tracy` (a lazy dependency's capture) and `zf` (unwired, the
   dependency of that name) resolve; `config` (options) is dropped.
2. Zig 0.11's `exe.addModule("helper", helper)` and a module's
   `.dependencies` resolve `helper` and `fmt` in `legacy/`.
