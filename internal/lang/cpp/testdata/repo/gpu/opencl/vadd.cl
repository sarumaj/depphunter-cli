#include "common.clh"
#include "missing.h"
// #include "commented.clh"

__kernel void vadd(__global const float *a, __global float *b) {
    int i = get_global_id(0);
    b[i] += a[i];
}

kernel void blur(global float *img, local float *tile) { }

float helper(float new) { return new; }
