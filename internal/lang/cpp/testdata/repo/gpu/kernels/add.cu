#include <cuda_runtime.h>
#include <cuda_fp16.h>
#include "cublas_v2.h"
#include <thrust/device_vector.h>
#include <thrust/sort.h>
#include <cub/cub.cuh>
#include <cooperative_groups.h>
#include <cooperative_groups/reduce.h>
#include <nvtx3/nvToolsExt.h>
#include <curand_kernel.h>
#include <sm_61_intrinsics.h>
#include "add.cuh"
#include "helper_cuda.h"
#include <cstdio>
// #include <cufft.h>
/* #include <cusparse.h> */
#if 0
#include <cudnn.h>
#endif

__constant__ float coef[16];

__device__ __forceinline__ float sq(float x) { return x * x; }

__global__ void add(const float* a, float* b, int n) {
    int i = blockIdx.x * blockDim.x + threadIdx.x;
    if (i < n) b[i] = a[i] + sq(b[i]);
}

template <typename T>
__global__ void __launch_bounds__(256) reduce(T* out) {}

extern "C" __global__ void scale(float* x) { x[threadIdx.x] *= 2.0f; }

#define THREADS 128
__launch_bounds__(THREADS) __global__ void bounded(float* x) {}
extern "C" __global__ void __launch_bounds__(THREADS, 2) bounded2(float* x) {}

void launch(float* a, float* b, int n) { add<<<(n + 255) / 256, 256>>>(a, b, n); }
