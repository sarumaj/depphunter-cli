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
	results := analyze(t)
	cuda := func(packageName string) lang.Target { return std(ecosystemCUDA, packageName) }
	// cSpell: disable
	langtest.CheckImports(t, results["gpu/kernels/add.cu"], map[string]lang.Target{
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
		`#include <cstdio>`:        std(ecosystemCppStd, "cstdio"),
		// Not captured: in comments and #if 0.
	})
	langtest.CheckImports(t, results["gpu/kernels/add.cuh"], map[string]lang.Target{
		`#include <cuda_runtime_api.h>`: cuda("cuda_runtime_api.h"),
	})
	// Kernels, device functions and host functions are functions; a launch is a call.
	langtest.CheckSymbols(t, results["gpu/kernels/add.cu"], map[string]string{
		"sq": "func", "add": "func", "reduce": "func", "scale": "func", "THREADS": "macro", "bounded": "func",
		"bounded2": "func", "launch": "func",
	})
	langtest.CheckSymbols(t, results["gpu/kernels/add.cuh"], map[string]string{"twice": "func"})
	// A host program: the OpenCL headers, SYCL's CL/ header is a library, Apple's
	// OpenCL framework is the Apple SDK's.
	langtest.CheckImports(t, results["gpu/host/main.cpp"], map[string]lang.Target{
		`#include <CL/cl.h>`:         std(ecosystemOpenCL, "CL/cl.h"),
		`#include <CL/opencl.hpp>`:   std(ecosystemOpenCL, "CL/opencl.hpp"),
		`#include <CL/sycl.hpp>`:     external("CL"),
		`#include <OpenCL/opencl.h>`: std(swift.AppleEcosystem, "OpenCL"),
		`#include <cuda.h>`:          cuda("cuda.h"),
		`#include "kernels/add.cuh"`: {Local: "gpu/kernels/add.cuh"},
	})
	// cSpell: enable
}

// Verifies: REQ-CPP-015
func TestOpenCL(t *testing.T) {
	results := analyze(t)
	langtest.CheckImports(t, results["gpu/opencl/vadd.cl"], map[string]lang.Target{
		`#include "common.clh"`: {Local: "gpu/opencl/common.clh"},
		`#include "missing.h"`:  {},
	})
	// OpenCL C is read as C: `new` is a parameter's name.
	langtest.CheckSymbols(t, results["gpu/opencl/vadd.cl"], map[string]string{"vadd": "func", "blur": "func", "helper": "func"})
	langtest.CheckSymbols(t, results["gpu/opencl/common.clh"], map[string]string{"Pair": "type"})
	if results["gpu/lisp/util.cl"] != nil {
		t.Error("a Common Lisp .cl file was claimed")
	}
	for _, f := range []*scan.File{{Path: "k.cl", Language: "OpenCL"}, {Path: "k.clh", Language: "OpenCL"}, {Path: "a.cu"}, {Path: "a.cuh"}, {Path: "a.metal"}} {
		if !(Plugin{}).Claims(f) {
			t.Errorf("%s not claimed", f.Path)
		}
	}
	if (Plugin{}).Claims(&scan.File{Path: "util.cl", Language: "Common Lisp"}) {
		t.Error("util.cl (Common Lisp) claimed")
	}
}

// Verifies: REQ-CPP-015, REQ-CPP-016
func TestMetal(t *testing.T) {
	results := analyze(t)
	apple := func(packageName string) lang.Target { return std(swift.AppleEcosystem, packageName) }
	langtest.CheckImports(t, results["gpu/metal/Shaders.metal"], map[string]lang.Target{
		`#include <metal_stdlib>`:  apple("Metal"),
		`#include <simd/simd.h>`:   apple("simd"),
		`#include "ShaderTypes.h"`: {Local: "gpu/metal/ShaderTypes.h"},
	})
	langtest.CheckImports(t, results["gpu/metal/ShaderTypes.h"], map[string]lang.Target{
		`#include <simd/simd.h>`: apple("simd"),
	})
	// Vertex, fragment and compute functions.
	langtest.CheckSymbols(t, results["gpu/metal/Shaders.metal"], map[string]string{
		"VertexOut": "struct", "vertexShader": "func", "fragmentShader": "func", "compute": "func",
	})
}

// Verifies: REQ-CPP-016
func TestSDK(t *testing.T) {
	for name, want := range map[string]lang.Target{
		"cuda_runtime.h":       {Ecosystem: ecosystemCUDA, Package: "cuda_runtime.h"},
		"cudnn_ops.h":          {Ecosystem: ecosystemCUDA, Package: "cudnn_ops.h"},
		"nppi_filtering.h":     {Ecosystem: ecosystemCUDA, Package: "nppi_filtering.h"},
		"cuda/std/atomic":      {Ecosystem: ecosystemCUDA, Package: "cuda"},
		"cub/device/scan.cuh":  {Ecosystem: ecosystemCUDA, Package: "cub"},
		"CL/cl2.hpp":           {Ecosystem: ecosystemOpenCL, Package: "CL/cl2.hpp"},
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
