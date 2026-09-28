---
id: REQ-SHADER-001
title: Shader files claimed
scope: shader
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The shader plugin **shall** claim GLSL files (`.glsl`, `.vert`, `.frag`,
`.geom`, `.tesc`, `.tese`, `.comp`, `.rgen`, `.rchit`, `.rahit`, `.rmiss`,
`.rint`, `.rcall`, `.vsh`, `.fsh`), HLSL files (`.hlsl`, `.hlsli`, `.fx`,
`.fxh`, and Unreal's `.usf` and `.ush`) and WGSL files (`.wgsl`, and WESL's
`.wesl`), in any case, and `.fs`, `.vs`, `.gs`, `.mesh` and `.task` files
only when the scan labels them GLSL (REQ-LANG-015). CUDA, OpenCL C and
Metal **shall** be left to the C/C++ plugin (REQ-CPP-015).

## Rationale

The shading languages are no C dialect a C/C++ scanner reads: GLSL's
interface blocks, HLSL's semantics, cbuffers and techniques, and WGSL's
syntax and module systems need their own reading. `.fs` is F#'s and
Forth's too, and `.vs`, `.gs`, `.mesh` and `.task` name other files (Google
Apps Script, mesh data, task lists); the scan's sniff of the head decides.

## Acceptance criteria

1. `shaders/glsl/pbr.frag`, `d3d/hlsl/Lighting.hlsl`, `effects/Blur.fx`,
   `Plugins/MyFX/Shaders/Private/Glow.usf`, `assets/shaders/custom.wgsl` and
   `crates/fx/src/lighting.wesl` are claimed.
2. `shaders/glsl/basic.vs` and `shaders/glsl/basic.fs`, labelled GLSL, are
   claimed; `src/App.fs` (F#), an unlabelled `.vs` or `.mesh` file, `.cu`,
   `.metal` and an OpenCL `.cl` file are not.
