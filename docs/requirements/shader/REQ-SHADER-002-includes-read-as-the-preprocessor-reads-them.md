---
id: REQ-SHADER-002
title: Includes read as the preprocessor reads them
scope: shader
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The shader plugin **shall** read the `#include "x"` and `#include <x>`
directives of GLSL (the GL_GOOGLE_include_directive and
GL_ARB_shading_language_include forms) and HLSL with the C/C++ plugin's
directive scanner (REQ-CPP-002, REQ-CPP-007): comments are removed,
continued lines joined, and the branches of `#if 0` skipped. Any other
condition **shall** be taken as live on every branch.

## Rationale

GLSL (through glslang's and shaderc's extensions) and HLSL (dxc, fxc) run
C's preprocessor; one scanner reads directives the same everywhere.

## Acceptance criteria

1. The five includes of `shaders/glsl/pbr.frag` are read; one in a line
   comment, one in a block comment and one in `#if 0` are not.
