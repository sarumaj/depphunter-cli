package cpp

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/lang/swift"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo/gpu: a CUDA kernel and its header, a vendored copy of CUB, an OpenCL
// kernel with a header of its own beside a Common Lisp file also ending in .cl, a
// host program including the OpenCL and CUDA headers, and Metal shaders sharing a
// header with the app.

// Verifies: REQ-CPP-015, REQ-CPP-016
func TestCUDA(t *testing.T) {
	res := analyze(t)
	cuda := func(pkg string) lang.Target { return std(ecoCUDA, pkg) }
	// cSpell: disable
	langtest.CheckImports(t, res["gpu/kernels/add.cu"], map[string]lang.Target{
		`#include <cuda_runtime.h>`:              cuda("cuda_runtime.h"),
		`#include <cuda_fp16.h>`:                 cuda("cuda_fp16.h"),
		`#include "cublas_v2.h"`:                 cuda("cublas_v2.h"),
		`#include <thrust/device_vector.h>`:      cuda("thrust"),
		`#include <thrust/sort.h>`:               cuda("thrust"),
		`#include <cooperative_groups.h>`:        cuda("cooperative_groups.h"),
		`#include <cooperative_groups/reduce.h>`: cuda("cooperative_groups"),
		`#include <nvtx3/nvToolsExt.h>`:          cuda("nvtx3"),
		`#include <curand_kernel.h>`:             cuda("curand_kernel.h"),
		`#include <sm_61_intrinsics.h>`:          cuda("sm_61_intrinsics.h"),
		// The project's own copy of CUB, found by the end of its path.
		`#include <cub/cub.cuh>`: {Local: "gpu/third_party/cub/cub/cub.cuh"},
		`#include "add.cuh"`:     {Local: "gpu/kernels/add.cuh"},
		// cuda-samples' helper, not in this project: generated or found elsewhere.
		`#include "helper_cuda.h"`: {},
		`#include <cstdio>`:        std(ecoCppStd, "cstdio"),
		// Not captured: in comments and #if 0.
	})
	langtest.CheckImports(t, res["gpu/kernels/add.cuh"], map[string]lang.Target{
		`#include <cuda_runtime_api.h>`: cuda("cuda_runtime_api.h"),
	})
	// Kernels, device functions and host functions are functions; a launch is a call.
	langtest.CheckSymbols(t, res["gpu/kernels/add.cu"], map[string]string{
		"sq": "func", "add": "func", "reduce": "func", "scale": "func", "THREADS": "macro", "bounded": "func",
		"bounded2": "func", "launch": "func",
	})
	langtest.CheckSymbols(t, res["gpu/kernels/add.cuh"], map[string]string{"twice": "func"})
	// A host program: the OpenCL headers, SYCL's CL/ header is a library, Apple's
	// OpenCL framework is the Apple SDK's.
	langtest.CheckImports(t, res["gpu/host/main.cpp"], map[string]lang.Target{
		`#include <CL/cl.h>`:         std(ecoOpenCL, "CL/cl.h"),
		`#include <CL/opencl.hpp>`:   std(ecoOpenCL, "CL/opencl.hpp"),
		`#include <CL/sycl.hpp>`:     external("CL"),
		`#include <OpenCL/opencl.h>`: std(swift.AppleEcosystem, "OpenCL"),
		`#include <cuda.h>`:          cuda("cuda.h"),
		`#include "kernels/add.cuh"`: {Local: "gpu/kernels/add.cuh"},
	})
	// cSpell: enable
}

// Verifies: REQ-CPP-015
func TestOpenCL(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["gpu/opencl/vadd.cl"], map[string]lang.Target{
		`#include "common.clh"`: {Local: "gpu/opencl/common.clh"},
		`#include "missing.h"`:  {},
	})
	// OpenCL C is read as C: `new` is a parameter's name.
	langtest.CheckSymbols(t, res["gpu/opencl/vadd.cl"], map[string]string{"vadd": "func", "blur": "func", "helper": "func"})
	langtest.CheckSymbols(t, res["gpu/opencl/common.clh"], map[string]string{"Pair": "type"})
	if res["gpu/lisp/util.cl"] != nil {
		t.Error("a Common Lisp .cl file was claimed")
	}
	for _, f := range []*scan.File{{Path: "k.cl", Lang: "OpenCL"}, {Path: "k.clh", Lang: "OpenCL"}, {Path: "a.cu"}, {Path: "a.cuh"}, {Path: "a.metal"}} {
		if !(Plugin{}).Claims(f) {
			t.Errorf("%s not claimed", f.Path)
		}
	}
	if (Plugin{}).Claims(&scan.File{Path: "util.cl", Lang: "Common Lisp"}) {
		t.Error("util.cl (Common Lisp) claimed")
	}
}

// Verifies: REQ-CPP-015, REQ-CPP-016
func TestMetal(t *testing.T) {
	res := analyze(t)
	apple := func(pkg string) lang.Target { return std(swift.AppleEcosystem, pkg) }
	langtest.CheckImports(t, res["gpu/metal/Shaders.metal"], map[string]lang.Target{
		`#include <metal_stdlib>`:  apple("Metal"),
		`#include <simd/simd.h>`:   apple("simd"),
		`#include "ShaderTypes.h"`: {Local: "gpu/metal/ShaderTypes.h"},
	})
	langtest.CheckImports(t, res["gpu/metal/ShaderTypes.h"], map[string]lang.Target{
		`#include <simd/simd.h>`: apple("simd"),
	})
	// Vertex, fragment and compute functions.
	langtest.CheckSymbols(t, res["gpu/metal/Shaders.metal"], map[string]string{
		"VertexOut": "struct", "vertexShader": "func", "fragmentShader": "func", "compute": "func",
	})
}

// Verifies: REQ-CPP-016
func TestSDK(t *testing.T) {
	for name, want := range map[string]lang.Target{
		"cuda_runtime.h":       {Ecosystem: ecoCUDA, Package: "cuda_runtime.h"},
		"cudnn_ops.h":          {Ecosystem: ecoCUDA, Package: "cudnn_ops.h"},
		"nppi_filtering.h":     {Ecosystem: ecoCUDA, Package: "nppi_filtering.h"},
		"cuda/std/atomic":      {Ecosystem: ecoCUDA, Package: "cuda"},
		"cub/device/scan.cuh":  {Ecosystem: ecoCUDA, Package: "cub"},
		"CL/cl2.hpp":           {Ecosystem: ecoOpenCL, Package: "CL/cl2.hpp"},
		"metal_raytracing":     {Ecosystem: swift.AppleEcosystem, Package: "Metal"},
		"cuda_interval.h":      {},
		"CL/sycl.hpp":          {},
		"helper_cuda.h":        {},
		"thrust.h":             {},
		"nvmedia_image.h":      {},
		"metal/metal_stdlib.h": {},
	} {
		got, _ := sdk(name)
		if got != want {
			t.Errorf("%s: got %+v, want %+v", name, got, want)
		}
	}
}
