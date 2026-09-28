package shader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo: GLSL shaders under shaders/ with includes beside, below, above
// and in include/, a GL_ARB named string, sniffed .vs/.fs shaders beside an F#
// file; DirectX-style HLSL under d3d/ with a header shared with C++, an effect
// file and an Unreal plugin's shaders; a Bevy game's WGSL assets (Cargo.toml
// declares bevy, Cargo.lock locks bevy_pbr) and a crate of WESL modules.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-SHADER-002, REQ-SHADER-004
func TestGLSL(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["shaders/glsl/pbr.frag"], map[string]lang.Target{
		`#include "common/lighting.glsl"`: {Local: "shaders/glsl/common/lighting.glsl"},
		`#include "lib/noise.glsl"`:       {Local: "shaders/lib/noise.glsl"},
		`#include <brdf.glsl>`:            {Local: "include/brdf.glsl"},
		// GL_ARB_shading_language_include names strings from the root.
		`#include "/shared/colors.glsl"`: {Local: "shared/colors.glsl"},
		// The application writes it: nothing to name.
		`#include "generated/defines.glsl"`: {},
		// Not captured: in comments and #if 0.
	})
	langtest.CheckImports(t, res["shaders/glsl/common/lighting.glsl"], map[string]lang.Target{
		`#include "../utils.glsl"`: {Local: "shaders/glsl/utils.glsl"},
	})
	// Found in a parent directory, ignoring case.
	langtest.CheckImports(t, res["shaders/glsl/post/blur.comp"], map[string]lang.Target{
		`#include "Utils.glsl"`: {Local: "shaders/glsl/utils.glsl"},
	})
	// .vs and .fs files the scan found GLSL in; the F# file is not claimed.
	langtest.CheckImports(t, res["shaders/glsl/basic.vs"], map[string]lang.Target{
		`#include "common/lighting.glsl"`: {Local: "shaders/glsl/common/lighting.glsl"},
	})
	if res["shaders/glsl/basic.fs"] == nil {
		t.Error("basic.fs (GLSL) not analyzed")
	}
	if res["src/App.fs"] != nil {
		t.Error("App.fs (F#) analyzed")
	}
}

// Verifies: REQ-SHADER-003
func TestGLSLSymbols(t *testing.T) {
	res := analyze(t)
	// Interface blocks, structs and functions; a prototype of a defined function is
	// not a second symbol, and initializer lists in braces are no bodies.
	langtest.CheckSymbols(t, res["shaders/glsl/pbr.frag"], map[string]string{
		"Camera": "block", "Lights": "block", "Material": "struct", "shade": "func", "main": "func",
	})
	langtest.CheckSymbols(t, res["shaders/glsl/common/lighting.glsl"], map[string]string{"attenuation": "func"})
	langtest.CheckSymbols(t, res["shaders/glsl/basic.fs"], map[string]string{"main": "func"})
}

// Verifies: REQ-SHADER-002, REQ-SHADER-003, REQ-SHADER-004, REQ-SHADER-009
func TestHLSL(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["d3d/hlsl/Lighting.hlsl"], map[string]lang.Target{
		`#include "Common.hlsli"`:            {Local: "d3d/hlsl/Common.hlsli"},
		`#include "..\Shared\SharedTypes.h"`: {Local: "d3d/Shared/SharedTypes.h"},
		`#include <Samplers.hlsli>`:          {Local: "d3d/include/Samplers.hlsli"},
	})
	// Written in another case than the file's.
	langtest.CheckImports(t, res["d3d/hlsl/Common.hlsli"], map[string]lang.Target{
		`#include "samplers.hlsli"`: {Local: "d3d/include/Samplers.hlsli"},
	})
	langtest.CheckSymbols(t, res["d3d/hlsl/Lighting.hlsl"], map[string]string{
		"SceneCB": "cbuffer", "VSIn": "struct", "PSIn": "struct", "VSMain": "func", "PSMain": "func",
		"CSMain": "func", "Util": "namespace", "Util.Sq": "func",
	})
	langtest.CheckImports(t, res["effects/Blur.fx"], map[string]lang.Target{
		`#include "Blur.fxh"`: {Local: "effects/Blur.fxh"},
	})
	langtest.CheckSymbols(t, res["effects/Blur.fx"], map[string]string{"PS": "func", "Blur": "technique"})
	langtest.CheckSymbols(t, res["effects/Blur.fxh"], map[string]string{})
	// Unreal's virtual paths: the plugin's own Shaders/ directory, else the engine.
	unreal := func(pkg string) lang.Target { return lang.Target{Ecosystem: ecoUnreal, Package: pkg} }
	langtest.CheckImports(t, res["Plugins/MyFX/Shaders/Private/Glow.usf"], map[string]lang.Target{
		`#include "/Engine/Private/Common.ush"`:                unreal("Engine"),
		`#include "/Plugin/MyFX/Private/GlowCommon.ush"`:       {Local: "Plugins/MyFX/Shaders/Private/GlowCommon.ush"},
		`#include "/Plugin/Niagara/Private/NiagaraCommon.ush"`: unreal("Niagara"),
		`#include "/Project/Missing.ush"`:                      {},
	})
	langtest.CheckSymbols(t, res["Plugins/MyFX/Shaders/Private/Glow.usf"], map[string]string{"MainPS": "func"})
}

// Verifies: REQ-SHADER-005, REQ-SHADER-006, REQ-SHADER-007, REQ-SHADER-009
func TestWGSL(t *testing.T) {
	res := analyze(t)
	pbr := lang.Target{Ecosystem: ecoCrates, Package: "bevy_pbr", Version: "0.14.2", Pinned: true}
	// Not in Cargo.lock: bevy's requirement, floating.
	render := lang.Target{Ecosystem: ecoCrates, Package: "bevy_render", Version: "0.14"}
	util := lang.Target{Local: "assets/shaders/util.wgsl"}
	langtest.CheckImports(t, res["assets/shaders/custom.wgsl"], map[string]lang.Target{
		`#import bevy_pbr::forward_io::VertexOutput`: pbr,
		`#import bevy_render::view::View`:            render,
		`#import bevy_render::globals::Globals`:      render,
		// Declared by #define_import_path game::util.
		`#import game::util::hash`: util,
		`#import game::util::PI`:   util,
		// Bevy's asset path, and a file embedded in a crate of the project.
		`#import "shaders/noise.wgsl"::fbm`:              {Local: "assets/shaders/noise.wgsl"},
		`#import "embedded://fx/render/view.wesl"::View`: {Local: "crates/fx/src/render/view.wesl"},
		`#import naga_missing::thing`:                    {},
		// Continued over lines; both branches of #ifdef.
		`#import bevy_pbr::mesh_functions`:     pbr,
		`#import bevy_pbr::forward_io::Vertex`: pbr,
		`#import bevy_pbr::skinning`:           pbr,
		// Not captured: in comments.
	})
	langtest.CheckSymbols(t, res["assets/shaders/custom.wgsl"], map[string]string{
		"material_color": "var", "SCALE": "const", "WORKGROUP": "const", "Params": "struct", "Color": "type",
		"fragment": "func",
	})
	langtest.CheckSymbols(t, res["assets/shaders/util.wgsl"], map[string]string{"PI": "const", "hash": "func"})
	view := lang.Target{Local: "crates/fx/src/render/view.wesl"}
	shapes := lang.Target{Local: "crates/fx/src/shapes.wesl"}
	langtest.CheckImports(t, res["crates/fx/src/lighting.wesl"], map[string]lang.Target{
		`import package::render::view::View`: view,
		`import super::shapes::circle`:       shapes,
		`import super::shapes::square`:       shapes,
		// The crate by its name, through the workspace's path dependency.
		`import fx::render::view`:                               view,
		`import bevy_pbr::mesh_functions::get_world_from_local`: pbr,
		`import package::missing::thing`:                        {},
	})
	langtest.CheckSymbols(t, res["crates/fx/src/lighting.wesl"], map[string]string{"light": "func"})
}

// Verifies: REQ-SHADER-001
func TestClaims(t *testing.T) {
	for _, f := range []scan.File{
		{Path: "a.glsl"}, {Path: "a.FRAG"}, {Path: "a.rchit"}, {Path: "a.hlsli"}, {Path: "a.usf"}, {Path: "a.wgsl"},
		{Path: "a.wesl"}, {Path: "a.fs", Lang: "GLSL"}, {Path: "a.vs", Lang: "GLSL"}, {Path: "a.mesh", Lang: "GLSL"},
	} {
		if !(Plugin{}).Claims(&f) {
			t.Errorf("%s not claimed", f.Path)
		}
	}
	for _, f := range []scan.File{
		{Path: "a.fs", Lang: "F#"}, {Path: "a.fs", Lang: "Forth"}, {Path: "a.vs"}, {Path: "a.mesh"}, {Path: "a.cu"},
		{Path: "a.metal"}, {Path: "a.cl", Lang: "OpenCL"}, {Path: "a.glsl", Binary: true},
	} {
		if (Plugin{}).Claims(&f) {
			t.Errorf("%s (%s) claimed", f.Path, f.Lang)
		}
	}
}

// Extraction returns for any input: every prefix of the fixtures and runs of
// unfinished constructs, in each dialect.
//
// Verifies: REQ-SHADER-008
func TestTruncated(t *testing.T) {
	var srcs []string
	filepath.WalkDir("testdata/repo", func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && dialect(p, "GLSL") != 0 {
			b, _ := os.ReadFile(p)
			for i := range b {
				srcs = append(srcs, string(b[:i]))
			}
		}
		return nil
	})
	for _, run := range []string{"{", "}", "(", ")", "[", "<", "/*", "\"", "#import a::{", "import a::{", "a::",
		"struct S {", "namespace N {", "uniform B {", "f(x) {", "@a(", "var<", "fn ", "= {", "#include \"", "\\\n"} {
		srcs = append(srcs, strings.Repeat(run, 200_000/len(run)))
	}
	for _, src := range srcs {
		for _, ext := range []string{".glsl", ".hlsl", ".wgsl"} {
			start := time.Now()
			if _, err := (Plugin{}).Extract(&scan.File{Path: "x" + ext}, []byte(src)); err != nil {
				t.Fatal(err)
			}
			if d := time.Since(start); d > 2*time.Second {
				t.Errorf("%s %.20q (%d bytes): %v", ext, src, len(src), d)
			}
		}
	}
}
