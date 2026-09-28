---
id: REQ-CPP-016
title: GPU toolkit header islands
scope: cpp
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The C/C++ plugin **shall** assign an include the project does not resolve
to `cuda` ("CUDA Toolkit") when it is a header of the CUDA Toolkit - the
runtime and driver APIs and their types (`cuda_runtime.h`, `cuda.h`,
`cuda_fp16.h`, `vector_types.h`, `cooperative_groups.h`, `mma.h`,
`nvrtc.h`, ...), a header of its libraries (cuBLAS, cuSPARSE, cuSOLVER,
cuFFT, cuRAND, cuDNN, CUPTI, NPP, NVTX 2), a per-architecture intrinsics
header (`sm_61_intrinsics.h`), each named as written, or a header in one of
its directories (`thrust/`, `cub/`, `cuda/`, `nv/`, `nvtx3/`,
`cooperative_groups/`, `crt/`), named after the directory - to `opencl`
("OpenCL headers") when it is in `CL/` (except SYCL's `CL/sycl*`), named as
written, and to the Apple SDKs island (`apple-sdk`) as `Metal` for the Metal
Shading Language's library (`metal_stdlib`, `metal_*`), as `simd` for
`simd/*` and as `OpenCL` for `OpenCL/*`. These islands **shall** be
standard-library islands, and **shall** be tried only after the project's
own files, including a file whose path ends in the include.

## Rationale

The toolkits are installed SDKs, not dependencies a manifest declares: like
the system headers they are hidden unless standard libraries are shown.
Without them a CUDA project's graph filled with a `c-external` library per
toolkit header (`cuda_runtime`, `curand_kernel`, `vector_types` ...).
Repositories that vendor Thrust or CUB (or are CCCL itself) include their
own copies.

## Acceptance criteria

1. `<cuda_runtime.h>` resolves to `cuda` `cuda_runtime.h`,
   `<thrust/sort.h>` to `cuda` `thrust`, `<curand_kernel.h>` to `cuda`
   `curand_kernel.h`, and `<cub/cub.cuh>` to the project's
   `gpu/third_party/cub/cub/cub.cuh`.
2. `<CL/cl.h>` resolves to `opencl` `CL/cl.h`, `<CL/sycl.hpp>` to
   `c-external` `CL`, and `<OpenCL/opencl.h>` to `apple-sdk` `OpenCL`.
3. `<metal_stdlib>` resolves to `apple-sdk` `Metal` and `<simd/simd.h>` to
   `apple-sdk` `simd`.
4. `cuda_interval.h`, `helper_cuda.h` and `nvmedia_image.h` are not toolkit
   headers.
