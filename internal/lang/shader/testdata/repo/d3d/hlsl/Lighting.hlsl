#include "Common.hlsli"
#include "..\Shared\SharedTypes.h"
#include <Samplers.hlsli>
// #include "Commented.hlsli"
#define RS "RootFlags(ALLOW_INPUT_ASSEMBLER_INPUT_LAYOUT)"

cbuffer SceneCB : register(b0)
{
    float4x4 viewProj;
};

Texture2D<float4> gTex : register(t0);
SamplerState gSamp : register(s0);

struct VSIn { float3 pos : POSITION; };
struct PSIn { float4 pos : SV_Position; };

[RootSignature(RS)]
PSIn VSMain(VSIn i)
{
    PSIn o;
    o.pos = mul(float4(i.pos, 1.0), viewProj);
    return o;
}

float4 PSMain(PSIn i) : SV_Target
{
    return gTex.Sample(gSamp, i.pos.xy);
}

[numthreads(8, 8, 1)]
void CSMain(uint3 id : SV_DispatchThreadID) {}

static const float kWeights[2] = { 0.5, 0.5 };

namespace Util
{
    float Sq(float x) { return x * x; }
}
