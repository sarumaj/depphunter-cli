import package::render::view::View;
import super::shapes::{circle, square as sq};
import fx::render::view;
import bevy_pbr::mesh_functions::get_world_from_local;
@if(FOO) import package::missing::thing;

fn light(v: View) -> f32 { return circle(1.0); }
