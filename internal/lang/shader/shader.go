// Package shader analyzes the shading languages: GLSL (.glsl, .vert, .frag, ...,
// and .fs, .vs, .gs, .mesh, .task when the scan found GLSL in them), HLSL (.hlsl,
// .hlsli, .fx, .fxh and Unreal's .usf, .ush) and WGSL with its module systems
// (.wgsl; WESL's .wesl). The C dialects of GPU programming - CUDA, OpenCL C and
// Metal - are the cpp plugin's, as they include C and C++ headers.
//
// GLSL and HLSL borrow C's preprocessor: `#include` is read by the cpp plugin's
// directive scanner and resolved as the cpp plugin resolves a project header (the
// includer's directory, include/ and src/, a unique file ending in the path), then
// against the includer's parent directories, ignoring case last. Unreal's virtual
// paths (/Engine/..., /Plugin/Name/...) link to the engine's or the plugin's
// Shaders/ directory when the repository has it, else to an "Unreal Engine
// shaders" island. Anything else a shader includes is a file the application or
// the engine hands the compiler: nothing to name.
//
// WGSL has no include; naga_oil (Bevy's) adds `#import a::b::item` and
// `#define_import_path a::b`, WESL `import package::a::item;`, `super::` and
// package names. A module path links to the file declaring it with
// #define_import_path, to the file its path names below the package root, or to
// a Rust crate the Cargo manifests declare (Bevy's bevy_pbr, bevy_render ...,
// which ship the shaders they import).
//
// Every language is read by a small scanner (clike.go, wgsl.go); the vendored
// grammars are not used (REQ-SHADER-008).
package shader

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemCrates = lang.EcosystemCrates
	ecosystemUnreal = "unreal-engine"
)

// Dialects the scanners tell apart.
const (
	glsl = iota + 1
	hlsl
	wgsl
)

// extensions maps the extensions the plugin claims to their dialect.
//
// Implements: REQ-SHADER-001
var extensions = map[string]int{
	".glsl": glsl, ".vert": glsl, ".frag": glsl, ".geom": glsl, ".tesc": glsl, ".tese": glsl,
	".comp": glsl, ".rgen": glsl, ".rchit": glsl, ".rahit": glsl, ".rmiss": glsl, ".rint": glsl,
	".rcall": glsl, ".vsh": glsl, ".fsh": glsl,
	".hlsl": hlsl, ".hlsli": hlsl, ".fx": hlsl, ".fxh": hlsl, ".usf": hlsl, ".ush": hlsl,
	".wgsl": wgsl, ".wesl": wgsl,
}

// sniffed are the extensions that are GLSL only when the scan says so: F# and Forth
// use .fs, and .vs, .gs, .mesh and .task name other files too.
var sniffed = map[string]bool{".fs": true, ".vs": true, ".gs": true, ".mesh": true, ".task": true}

// Implements: REQ-SHADER-001
type Plugin struct{}

func (Plugin) Name() string { return "shader" }
func (Plugin) Version() int { return 1 }

// Implements: REQ-SHADER-001
func (Plugin) Claims(f *scan.File) bool { return !f.Binary && dialect(f.Path, f.Language) != 0 }

// dialect is the language of a file, 0 for a file the plugin does not read.
func dialect(p, label string) int {
	extension := strings.ToLower(path.Ext(p))
	if sniffed[extension] {
		if label == "GLSL" {
			return glsl
		}
		return 0
	}
	return extensions[extension]
}

// Ecosystems: crates.io, the Rust crates shipping the WGSL modules a shader
// imports, and Unreal Engine's shader directories.
//
// Implements: REQ-SHADER-004, REQ-SHADER-007
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemCrates, Name: "crates.io"},
		{ID: ecosystemUnreal, Name: "Unreal Engine shaders", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Extract reads the includes or imports and the definitions. Which scanner reads a
// file depends only on its extension; a sniffed extension (.fs) is claimed only as
// GLSL, so the cached extraction cannot differ with the label.
//
// Implements: REQ-SHADER-002, REQ-SHADER-003, REQ-SHADER-005, REQ-SHADER-008
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	d := extensions[strings.ToLower(path.Ext(f.Path))]
	if d == wgsl {
		return extractWGSL(source), nil
	}
	if d == 0 {
		d = glsl
	}
	return extractC(source, d), nil
}
