#version 330 core
out vec4 color;
uniform sampler2D tex;
void main() { color = texture(tex, vec2(0.0)); }
