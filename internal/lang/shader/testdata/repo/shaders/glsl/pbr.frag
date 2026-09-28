#version 450
#extension GL_GOOGLE_include_directive : require
#include "common/lighting.glsl"
#include "lib/noise.glsl"
#include <brdf.glsl>
#include "/shared/colors.glsl"
#include "generated/defines.glsl"
// #include "commented.glsl"
/* #include "blocked.glsl" */
#if 0
#include "dead.glsl"
#endif

layout(std140, binding = 0) uniform Camera {
    mat4 view;
} cam;

layout(set = 1, binding = 0) buffer Lights { vec4 l[]; };

struct Material {
    vec3 albedo;
    float rough;
};

layout(location = 0) in vec3 inNormal;
layout(location = 0) out vec4 outColor;

const float weights[3] = float[](0.2, 0.5, 0.3);
const vec2 offsets[2] = { vec2(0.0), vec2(1.0) };

vec3 shade(Material m, vec3 n);

vec3 shade(Material m, vec3 n) {
    return m.albedo * max(dot(n, vec3(0.0, 1.0, 0.0)), 0.0);
}

void main() {
    outColor = vec4(shade(Material(vec3(1.0), 0.5), inNormal), 1.0);
}
