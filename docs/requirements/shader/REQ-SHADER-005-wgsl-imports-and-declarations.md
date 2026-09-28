---
id: REQ-SHADER-005
title: WGSL imports and declarations read
scope: shader
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The shader plugin **shall** read naga_oil's `#import` directives (continued
over lines while their braces are open) and WESL's `import` statements
(after attributes such as `@if(X)`), expanding an import tree
(`a::{b, c::{d}}`, `a::b as c`, naga_oil's quoted file `"x.wgsl"::item`)
into one import per path, `#ifdef` branches all read and comments (nested
block comments included) skipped. The module-scope declarations **shall**
be symbols: `fn` (`func`), `struct`, `var` (`var`), `const` and `override`
(`const`) and `alias` (`type`).

## Rationale

WGSL has no include; Bevy's naga_oil and WESL add module imports that are
the edges between shader files.

## Acceptance criteria

1. The multi-line `#import bevy_pbr::{mesh_functions, forward_io::Vertex}`
   of `assets/shaders/custom.wgsl` gives two imports, the one in `#ifdef
   SKINNED` is read, and those in comments are not.
2. `custom.wgsl` declares `material_color`, `SCALE`, `WORKGROUP`, `Params`,
   `Color` and `fragment`; a function in a nested comment is none.
