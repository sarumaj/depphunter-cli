#pragma OPENCL EXTENSION cl_khr_fp64 : enable
__kernel void add(__global const float *a, __global float *b) {
  b[get_global_id(0)] += a[get_global_id(0)];
}
