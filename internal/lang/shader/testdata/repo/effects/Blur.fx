#include "Blur.fxh"
float4 PS(float4 p : SV_Position) : SV_Target { return p; }
technique11 Blur
{
    pass P0
    {
        SetPixelShader(CompileShader(ps_5_0, PS()));
    }
}
