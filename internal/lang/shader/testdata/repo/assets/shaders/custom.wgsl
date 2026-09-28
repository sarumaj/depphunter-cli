#import bevy_pbr::forward_io::VertexOutput
#import bevy_render::{view::View, globals::Globals}
#import game::util::{hash, PI}
#import "shaders/noise.wgsl"::fbm
#import naga_missing::thing
#import "embedded://fx/render/view.wesl"::View
#import bevy_pbr::{
    mesh_functions,   // the module itself
    forward_io::Vertex,
}
// #import fake::module
/* #import fake2::module */
#ifdef SKINNED
#import bevy_pbr::skinning
#endif

@group(2) @binding(0) var<uniform> material_color: vec4<f32>;
const SCALE: f32 = 2.0;
override WORKGROUP: u32 = 64;
struct Params {
    a: f32,
}
alias Color = vec4<f32>;

/* a nested /* comment */ fn hidden() {} */

@fragment
fn fragment(in: VertexOutput) -> @location(0) vec4<f32> {
    return material_color * SCALE;
}
