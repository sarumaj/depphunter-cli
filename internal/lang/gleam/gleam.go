// Package gleam analyzes Gleam packages: modules (.gleam), gleam.toml and the
// manifest.toml Gleam writes beside it.
//
// `import a/b/c` names the module src/a/b/c.gleam (or test/, dev/) of the
// importing package, of a path dependency, or of a Hex package. Gleam packages
// are Hex packages, so they share the hex island - its pinning rule, OSV's Hex
// advisories and the hex.pm client of --online - with Elixir and Erlang:
// gleam/list is gleam_stdlib, gleam/erlang/process gleam_erlang, lustre/element
// lustre (resolve.go). @external(erlang, "mod", "f") is an edge to the Erlang
// module - a project .erl file, Erlang/OTP or a package - and
// @external(javascript, "./ffi.mjs", "f") to that file. gleam.toml's
// dependencies and manifest.toml's packages are imports of what they name, the
// manifest pinning them and giving --resolve-depth their requirements.
//
// Gleam is read by a small lexer of its own (source.go), not the vendored
// tree-sitter grammar (REQ-GLEAM-010).
package gleam

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/beam"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The islands are the BEAM plugin's, so a Gleam package and the same package
// reached from Elixir or Erlang are one node.
const (
	ecoHex = "hex"
	ecoOTP = "erlang-std"
	ecoNPM = "npm"
)

const (
	classConfig   = "config"
	classManifest = "manifest"
)

// Implements: REQ-GLEAM-001
type Plugin struct{}

func (Plugin) Name() string { return "gleam" }
func (Plugin) Version() int { return 1 }

// Claims takes Gleam modules, gleam.toml and manifest.toml (in lower case: Julia's
// is Manifest.toml), except what Gleam downloads and compiles into build/.
//
// Implements: REQ-GLEAM-001
func (p Plugin) Claims(f *scan.File) bool {
	if f.Binary || inBuild(f.Path) {
		return false
	}
	return path.Ext(f.Path) == ".gleam" || p.Class(f) != ""
}

// inBuild reports whether a path is inside Gleam's build directory:
// build/packages/ (downloaded sources) or build/dev/, build/prod/, build/lsp/
// (compiled output). A build/ directory of any other layout is left alone.
func inBuild(p string) bool {
	segments := strings.Split(p, "/")
	for i := 0; i+2 < len(segments); i++ {
		if segments[i] == "build" {
			switch segments[i+1] {
			case "packages", "dev", "prod", "lsp":
				return true
			}
		}
	}
	return false
}

// Class tells gleam.toml and manifest.toml apart from other TOML files.
//
// Implements: REQ-GLEAM-001
func (Plugin) Class(f *scan.File) string { return class(f.Path) }

func class(p string) string {
	switch path.Base(p) {
	case "gleam.toml":
		return classConfig
	case "manifest.toml":
		return classManifest
	}
	return ""
}

// Ecosystems are the BEAM plugin's Hex and Erlang/OTP islands, and npm for the
// packages JavaScript externals import.
func (Plugin) Ecosystems() []lang.Ecosystem {
	var out []lang.Ecosystem
	for _, e := range (beam.Plugin{}).Ecosystems() {
		if e.ID == ecoHex || e.ID == ecoOTP {
			out = append(out, e)
		}
	}
	return append(out, lang.Ecosystem{ID: ecoNPM, Name: "npm"})
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-GLEAM-002, REQ-GLEAM-003, REQ-GLEAM-005, REQ-GLEAM-006
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	switch class(f.Path) {
	case classConfig:
		return extractConfig(src), nil
	case classManifest:
		return extractManifest(src), nil
	}
	return extractSource(src), nil
}
