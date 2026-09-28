---
id: REQ-SHADER-009
title: Read without a shader compiler
scope: shader
type: limitation
priority: should
status: implemented
verification:
  - inspection
---

## Statement

Shaders **shall** be read without a shader compiler or engine: preprocessor
and naga_oil conditions other than `#if 0` are not evaluated (every branch
counts, and a function whose signature differs per branch may hide the
definitions after it), macros are not expanded, the include directories a
build passes (`-I`) are known only from a compilation database, the
virtual paths an engine maps other than Unreal's are not known, a WGSL
item used by its full path without an import (`module::item`) is no
import, and GLSL-named files holding HLSL (compiled with `-x hlsl`) are
read as GLSL.

## Rationale

What a shader includes and defines is often decided by the engine at run
time (permutations, material graphs).

## Acceptance criteria

1. A `#ifdef` branch's `#import` is read; `/Project/Missing.ush` and an
   include of a generated file are dropped.
