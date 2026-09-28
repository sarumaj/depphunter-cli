---
id: REQ-SHADER-006
title: WGSL modules resolved to project files
scope: shader
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The shader plugin **shall** resolve a WGSL module path to a project file:
`package::a::b` to the module file (`.wesl`, else `.wgsl`) of the longest
prefix of `a::b` below the nearest parent directory of the importer that
has one, `super::` to the importer's directory (one more parent per
`super`), `self::` below the importer's own module, a path whose longest
prefix some file declares with naga_oil's `#define_import_path` to that
file (the closest one when several do), and a quoted file beside the
importer, below an `assets/` directory or directly in one of its parents,
or the only file whose path ends in it (`embedded://crate/x` below that
crate's `src/`). An unresolved path **shall** be tried as a crate
(REQ-SHADER-007) and otherwise dropped.

## Rationale

naga_oil declares a module's path in the file; WESL derives it from the
file's place below the package root, which is Bevy's crate `src/`.

## Acceptance criteria

1. `#import game::util::hash` resolves to `assets/shaders/util.wgsl`
   (`#define_import_path game::util`), and `"shaders/noise.wgsl"::fbm` to
   `assets/shaders/noise.wgsl`.
2. In `crates/fx/src/lighting.wesl`, `package::render::view::View` and
   `fx::render::view` resolve to `crates/fx/src/render/view.wesl`,
   `super::shapes::circle` to `crates/fx/src/shapes.wesl`, and
   `package::missing::thing` is dropped.
