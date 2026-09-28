// Package julia analyzes Julia sources (.jl) with Pkg's files: Project.toml (or
// JuliaProject.toml), Manifest.toml (or JuliaManifest.toml and the versioned
// Manifest-v1.11.toml) and Artifacts.toml.
//
// A `using` or `import` names a module. A relative one (using .Sub, ..Parent)
// resolves to the file defining that module, found through the include() graph of
// the package; an absolute one to the package's own entry file (src/Name.jl) or one
// of its submodules, a dependency the nearest Project.toml declares (a local
// package by its UUID or a [sources] path, else a Julia package pinned by the
// manifest or floating on its [compat] entry), or Julia's standard library (the
// hidden julia-std island). include("x.jl") is an edge to the file (resolve.go).
//
// Julia is read by a lexer of its own (lex.go), not the vendored tree-sitter
// grammar, which was slow and lost files to errors (REQ-JULIA-011).
package julia

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemJulia = "julia"
	ecosystemStd   = "julia-std"
)

// Kinds of claimed files; Class names them.
const (
	classSource    = "source"
	classProject   = "project"
	classManifest  = "manifest"
	classArtifacts = "artifacts"
)

// Implements: REQ-JULIA-001
type Plugin struct{}

func (Plugin) Name() string { return "julia" }
func (Plugin) Version() int { return 1 }

// fileClass tells the claimed files apart by name.
func fileClass(p string) string {
	base := path.Base(p)
	switch {
	case path.Ext(base) == ".jl":
		return classSource
	case base == "Project.toml" || base == "JuliaProject.toml":
		return classProject
	case manifestName(base):
		return classManifest
	case base == "Artifacts.toml" || base == "JuliaArtifacts.toml":
		return classArtifacts
	}
	return ""
}

// manifestName reports whether a file name is a manifest's: Manifest.toml,
// JuliaManifest.toml or a versioned Manifest-v1.11.toml.
func manifestName(base string) bool {
	stem, ok := strings.CutSuffix(base, ".toml")
	if !ok {
		return false
	}
	stem = strings.TrimPrefix(stem, "Julia")
	if stem == "Manifest" {
		return true
	}
	v, ok := strings.CutPrefix(stem, "Manifest-v")
	return ok && v != "" && strings.Trim(v, "0123456789.") == ""
}

// Claims takes Julia sources and Pkg's files.
//
// Implements: REQ-JULIA-001
func (Plugin) Claims(f *scan.File) bool {
	return !f.Binary && fileClass(f.Path) != ""
}

// Class keeps Project.toml, manifests and Artifacts.toml apart: they share .toml.
//
// Implements: REQ-JULIA-001
func (Plugin) Class(f *scan.File) string { return fileClass(f.Path) }

func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemJulia, Name: "Julia"},
		{ID: ecosystemStd, Name: "Julia standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Extract dispatches on the kind of file.
//
// Implements: REQ-JULIA-002, REQ-JULIA-003, REQ-JULIA-006, REQ-JULIA-007
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	switch fileClass(f.Path) {
	case classProject:
		return extractProject(source), nil
	case classManifest:
		return extractManifest(source), nil
	case classArtifacts:
		return extractArtifacts(source), nil
	}
	s := readSource(source)
	symbols := s.symbols
	if symbols == nil {
		symbols = []lang.Symbol{}
	}
	return &lang.Extraction{Imports: s.imports, Symbols: symbols}, nil
}
