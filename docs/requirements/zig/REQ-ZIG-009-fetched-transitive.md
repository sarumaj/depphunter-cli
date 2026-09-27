---
id: REQ-ZIG-009
uuid: 0d0945e4-c0db-423a-948d-76b09f00d762
title: Dependencies of fetched packages
scope: zig
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--resolve-depth`, a Zig package's dependencies **shall** be the URL
dependencies of its own `build.zig.zon` when Zig has fetched it: under
`zig-pkg/<hash>/` beside a `build.zig.zon` of the repository or above it
(Zig 0.16), else in the global cache's `p/<hash>/` (`$ZIG_GLOBAL_CACHE_DIR`,
`$XDG_CACHE_HOME/zig`, `%LOCALAPPDATA%\zig` on Windows, `~/.cache/zig`),
read from disk; such an answer counts as installed.

## Rationale

Zig has no lock file beyond the hashes; what it fetched is the only record
of the graph below the first level.

## Acceptance criteria

1. The fixture's `zig-pkg/vaxis-0.5.1-.../build.zig.zon` gives vaxis's
   `zigimg`; a package in `$ZIG_GLOBAL_CACHE_DIR/p/<hash>` counts as
   installed; a package not fetched has no dependencies.
