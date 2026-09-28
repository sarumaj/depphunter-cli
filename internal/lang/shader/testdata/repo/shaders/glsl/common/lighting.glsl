#ifndef LIGHTING_GLSL
#define LIGHTING_GLSL
#include "../utils.glsl"

#define PI 3.14159265

float attenuation(float d) { return 1.0 / (d * d); }

#endif
