---
id: REQ-GLEAM-004
title: Modules resolved to files and packages
scope: gleam
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Gleam plugin **shall** resolve a module path, in order: to the importing
package's `src/`, `test/` or `dev/` file (the package being the nearest
directory with a `gleam.toml`); to a module of a path dependency in the
repository; to the package gleam downloaded into `build/packages/` that
provides it, when that directory is on disk; to the declared or locked Hex
package named by the longest run of its leading segments joined by `_`
(`gleam/erlang/process` is `gleam_erlang`, `lustre/element` is `lustre`); a
`gleam/` module of the standard library to `gleam_stdlib` (a real, versioned
Hex package, not a hidden standard library); the built-in `gleam` prelude and
a missing module of the package's own namespace are dropped; anything else is
an unresolved Hex package named by the same rules (`gleam/community/ansi` is
`gleam_community_ansi`).

## Rationale

Gleam modules are named by their path under src/, and package names follow
their leading segments by convention; gleam_stdlib is pinned in manifest.toml
like any other package.

## Acceptance criteria

1. `gleam/list` is `gleam_stdlib` 0.40.0 as the manifest locks it,
   `inventory/stock` is `libs/inventory/src/inventory/stock.gleam`, and
   `support/fixtures` from a test is `dev/support/fixtures.gleam`.
2. `shop/missing` in package `shop` is dropped; `nothere/thing` is the
   unresolved package `nothere`.
3. With `build/packages/pretty_print/src/pp/doc.gleam` on disk, `pp/doc` is
   `pretty_print`.
