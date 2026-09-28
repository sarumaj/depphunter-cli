---
id: REQ-SHADER-007
title: Rust crates shipping WGSL modules
scope: shader
type: functional
priority: may
status: implemented
verification:
  - unit
---

## Statement

A WGSL module path the project's files do not declare **shall** resolve,
by its first segment, to the crate a Cargo manifest of the project declares
under that name (the one governing the importer first, then any other): a
crate of the workspace or a path dependency to the module file below its
`src/` (else the crate's directory), a crates.io dependency to the `crates`
package with the version Cargo.lock holds, as the Rust plugin names it
(REQ-RS-004, REQ-RS-007). A `bevy_*` crate that no manifest declares while
one declares `bevy` **shall** resolve to that crates.io package, pinned to
the version Cargo.lock holds, else floating on bevy's requirement. Any
other path **shall** be dropped.

## Rationale

Bevy's crates (bevy_pbr, bevy_render ...) ship the WGSL modules a game's
shaders import, and are released together with the bevy crate the game
declares; the import is a use of that crate.

## Acceptance criteria

1. `#import bevy_pbr::forward_io::VertexOutput` resolves to `crates`
   `bevy_pbr` 0.14.2, pinned by Cargo.lock; `bevy_render::view::View`
   (not locked) to `bevy_render` 0.14, floating.
2. `#import naga_missing::thing` is dropped.
