---
id: REQ-CPP-015
title: CUDA, Metal and OpenCL files claimed
scope: cpp
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The C/C++ plugin **shall** analyze CUDA (`.cu`, `.cuh`) and Metal
(`.metal`) files as C++, and OpenCL C kernels (`.clh`, and `.cl` files the
scan labels OpenCL, REQ-LANG-015) as C, with the includes, resolution and
definitions of C and C++: kernels (`__global__`, `__kernel`, `kernel`,
`vertex`, `fragment`) and device functions are functions, CUDA's launch
attributes (`__launch_bounds__(N)`) are stepped over, and a kernel launch
(`k<<<grid, block>>>(...)`) is a call. A `.cl` file the scan labels Common
Lisp **shall not** be claimed.

## Rationale

The GPU languages are dialects of C and C++: a `.cu` file includes C++
headers and the project's own `.cuh` and `.h` files, a Metal shader shares a
header with the Objective-C or Swift app, an OpenCL kernel includes its own
headers. One include path serves them all. The C/C++ scanner already reads
their definitions; OpenCL C, like C, may use C++ keywords as names.

## Acceptance criteria

1. `gpu/kernels/add.cu` includes `add.cuh` beside it and the project's copy
   of CUB, and defines `sq`, `add`, `reduce`, `scale`, `bounded`,
   `bounded2` and `launch`.
2. `gpu/opencl/vadd.cl` includes `common.clh` beside it and defines `vadd`,
   `blur` and `helper` (whose parameter is named `new`); the Common Lisp
   `gpu/lisp/util.cl` is not claimed.
3. `gpu/metal/Shaders.metal` includes `ShaderTypes.h` beside it and defines
   `VertexOut`, `vertexShader`, `fragmentShader` and `compute`.

## Notes

As in C++, an all-capitals function name reads as a test macro and is no
symbol (a kernel named `KNN`); a vendored toolkit copy is the project's
(REQ-CPP-016).
