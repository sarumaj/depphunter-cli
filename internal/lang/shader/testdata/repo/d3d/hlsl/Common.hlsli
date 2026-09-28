#ifndef COMMON_HLSLI
#define COMMON_HLSLI
#include "samplers.hlsli"
float3 Tonemap(float3 c) { return c / (1.0 + c); }
#endif
