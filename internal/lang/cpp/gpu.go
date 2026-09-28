package cpp

import (
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/swift"
)

// Islands of the GPU toolkits' headers. Like the system headers they come with an
// installed SDK rather than a package manifest, so both are standard-library
// islands, hidden unless standard libraries are shown.
const (
	ecoCUDA   = "cuda"
	ecoOpenCL = "opencl"
)

// cudaHeaders are the CUDA Toolkit's headers included by name (runtime and driver
// APIs, their types, the math libraries' entry points). The libraries with many
// headers are matched by cudaLibrary; thrust/, cub/ and the others by cudaDirs.
var cudaHeaders = set(`cuda.h cudaGL.h cudaEGL.h cudaGLTypedefs.h cudaEGLTypedefs.h
cudaTypedefs.h cudaProfiler.h cudaProfilerTypedefs.h cudaVDPAU.h cudaD3D9.h cudaD3D10.h
cudaD3D11.h cudla.h cudaNvSci.h
cuda_runtime.h cuda_runtime_api.h cuda_device_runtime_api.h cuda_fp16.h cuda_fp16.hpp
cuda_bf16.h cuda_bf16.hpp cuda_fp8.h cuda_fp8.hpp cuda_fp6.h cuda_fp6.hpp cuda_fp4.h
cuda_fp4.hpp cuda_texture_types.h cuda_surface_types.h cuda_gl_interop.h
cuda_egl_interop.h cuda_vdpau_interop.h cuda_d3d9_interop.h cuda_d3d10_interop.h
cuda_d3d11_interop.h cuda_profiler_api.h cuda_occupancy.h cuda_awbarrier.h
cuda_awbarrier_primitives.h cuda_awbarrier_helpers.h cuda_pipeline.h
cuda_pipeline_primitives.h cuda_pipeline_helpers.h cuda_stdint.h
device_launch_parameters.h device_functions.h device_atomic_functions.h
device_double_functions.h device_types.h vector_types.h vector_functions.h
vector_functions.hpp driver_types.h driver_functions.h host_defines.h host_config.h
builtin_types.h channel_descriptor.h texture_types.h surface_types.h
texture_fetch_functions.h texture_indirect_functions.h surface_functions.h
surface_indirect_functions.h math_constants.h math_functions.h library_types.h
common_functions.h cooperative_groups.h mma.h nvrtc.h nvml.h nvjpeg.h nvJitLink.h
nvPTXCompiler.h nvvm.h nvblas.h nvfunctional fatbinary_section.h nccl.h cutensor.h
cufile.h`)

// cudaLibrary matches the headers of the toolkit's libraries (cuBLAS, cuSPARSE,
// cuSOLVER, cuFFT, cuRAND, cuDNN, CUPTI, NPP, NVTX 2) and the per-architecture
// intrinsics (sm_61_intrinsics.h).
var cudaLibrary = regexp.MustCompile(`^(?:(?:cublas|cusparse|cusolver|cufft|curand|cudnn|cupti|npp|nvToolsExt)\w*|sm_\d+_\w+)\.h(?:pp)?$`)

// cudaDirs are the toolkit's header directories: Thrust, CUB and libcu++ (cuda/,
// nv/), NVTX 3, cooperative groups and the compiler's crt/.
var cudaDirs = set(`thrust cub cuda nv nvtx3 cooperative_groups crt`)

// metalHeader matches the Metal Shading Language's library headers (metal_stdlib,
// metal_math, metal_raytracing ...).
var metalHeader = regexp.MustCompile(`^metal_\w+$`)

// sdk returns the island of a header a GPU toolkit or the Metal SDK provides: the
// CUDA Toolkit (a bare header named as written, a directory by its name:
// <thrust/sort.h> -> thrust), the OpenCL headers (<CL/cl.h>, as written), and the
// Apple SDKs (<metal_stdlib> is Metal's, <simd/simd.h> simd's, <OpenCL/opencl.h>
// the OpenCL framework's). The project's own files are looked up first, so a
// repository vendoring Thrust or CUB links to its copy.
//
// Implements: REQ-CPP-016
func sdk(name string) (lang.Target, bool) {
	dir, _, nested := strings.Cut(name, "/")
	switch {
	case !nested && (cudaHeaders[name] || cudaLibrary.MatchString(name)):
		return lang.Target{Ecosystem: ecoCUDA, Package: name}, true
	case nested && cudaDirs[dir]:
		return lang.Target{Ecosystem: ecoCUDA, Package: dir}, true
	case nested && dir == "CL" && !strings.HasPrefix(name, "CL/sycl"): // <CL/sycl.hpp> is SYCL's
		return lang.Target{Ecosystem: ecoOpenCL, Package: name}, true
	case !nested && metalHeader.MatchString(name):
		return lang.Target{Ecosystem: swift.AppleEcosystem, Package: "Metal"}, true
	case nested && (dir == "simd" || dir == "OpenCL"):
		return lang.Target{Ecosystem: swift.AppleEcosystem, Package: dir}, true
	}
	return lang.Target{}, false
}
