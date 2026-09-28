#include <metal_stdlib>
#include <simd/simd.h>
#include "ShaderTypes.h"

using namespace metal;

struct VertexOut {
    float4 position [[position]];
};

vertex VertexOut vertexShader(uint vid [[vertex_id]], constant Uniforms &u [[buffer(0)]]) {
    VertexOut o;
    return o;
}

fragment float4 fragmentShader(VertexOut in [[stage_in]]) { return float4(1); }

kernel void compute(device float *out [[buffer(0)]], uint id [[thread_position_in_grid]]) { out[id] = 0; }
