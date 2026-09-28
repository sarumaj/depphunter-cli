---
id: REQ-SHADER-003
title: GLSL and HLSL definitions extracted
scope: shader
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The shader plugin **shall** record, at file scope and in HLSL namespaces
(as `Namespace.name`), the functions defined with a return type and a body
(entry points with HLSL semantics and attributes such as
`[numthreads(8, 8, 1)]` included; kind `func`), structs (`struct`), GLSL
interface blocks - a body after a storage qualifier such as `uniform`,
`buffer`, `in`, `out`, `shared` or a ray tracing one (`block`) - HLSL
`cbuffer` and `tbuffer` blocks (`cbuffer`), effect techniques
(`technique`, `technique10`, `technique11`; `technique`) and namespaces. A
prototype, a variable, an initializer list in braces and anything in a
function body or in `#if 0` **shall not** be a symbol.

## Rationale

These are what other shaders and the application refer to: entry points,
the structs and blocks shared with the host code.

## Acceptance criteria

1. `shaders/glsl/pbr.frag` defines `Camera` and `Lights` (blocks),
   `Material` (struct), `shade` and `main`.
2. `d3d/hlsl/Lighting.hlsl` defines `SceneCB` (cbuffer), `VSIn`,
   `PSIn`, `VSMain`, `PSMain`, `CSMain`, `Util` (namespace) and `Util.Sq`;
   `effects/Blur.fx` defines `PS` and the technique `Blur`.
