// Package fsharp analyzes F#: sources (.fs, .fsi, .fsx), project files (.fsproj)
// and Paket's paket.dependencies, paket.lock and paket.references.
//
// F# compiles a project's files in the order its .fsproj lists them, and a file can
// only use what earlier files declare. `open X.Y` and qualified names in code
// (Cart.add) resolve to the project files declaring that namespace, module or type
// - files earlier in compile order, or in a referenced project - else to a NuGet
// package, FSharp.Core, the .NET base library or an unresolved package
// (resolve.go). Packages come from internal/lang/nuget, which the C# plugin reads
// too, so a package both languages use is one node.
//
// F# is read by a small lexer and an offside-rule scanner (lex.go, source.go), not
// the vendored tree-sitter grammar (REQ-FSHARP-011), and without type checking:
// unqualified names are not linked (REQ-FSHARP-012).
package fsharp

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/nuget"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	classProject      = "project"
	classDependencies = "dependencies"
	classLock         = "lock"
	classReferences   = "references"
)

// Implements: REQ-FSHARP-001
type Plugin struct{}

func (Plugin) Name() string { return "fsharp" }
func (Plugin) Version() int { return 1 }

// Claims takes F# sources, .fsproj files and Paket's files. A .fs file is claimed
// only when scan labeled it F#: GLSL shaders and Forth use .fs too (scan tells
// them apart by content). What Paket downloads (paket-files/, packages/ beside a
// paket.dependencies) and FAKE's .fake/ cache are not read.
//
// Implements: REQ-FSHARP-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	switch extension := strings.ToLower(path.Ext(f.Path)); {
	case extension == ".fs":
		if f.Language != "F#" {
			return false
		}
	case extension == ".fsi" || extension == ".fsx" || extension == ".fsscript" || class(f.Path) != "":
	default:
		return false
	}
	return !downloaded(f)
}

// downloaded reports whether f lies where Paket or FAKE put what they fetched.
func downloaded(f *scan.File) bool {
	segments := strings.Split(f.Path, "/")
	for i, s := range segments[:len(segments)-1] {
		switch s {
		case "paket-files", ".fake", ".paket":
			return true
		case "packages":
			if f.AbsolutePath != "" && strings.HasSuffix(filepath.ToSlash(f.AbsolutePath), f.Path) {
				base := f.AbsolutePath[:len(f.AbsolutePath)-len(f.Path)]
				if hasPaket(filepath.Join(base, filepath.FromSlash(strings.Join(segments[:i], "/")))) {
					return true
				}
			}
		}
	}
	return false
}

var paketDirectories sync.Map // absolute directory -> it has a paket.dependencies

func hasPaket(directory string) bool {
	if v, ok := paketDirectories.Load(directory); ok {
		return v.(bool)
	}
	// Lstat: a marker committed as a symbolic link says nothing of its target.
	_, err := os.Lstat(filepath.Join(directory, "paket.dependencies"))
	paketDirectories.Store(directory, err == nil)
	return err == nil
}

// Class tells project files and Paket's files apart from sources and from other
// files sharing their extensions (.lock, .references).
//
// Implements: REQ-FSHARP-001
func (Plugin) Class(f *scan.File) string { return class(f.Path) }

func class(p string) string {
	switch base := path.Base(p); {
	case strings.HasSuffix(strings.ToLower(base), ".fsproj"):
		return classProject
	case base == "paket.dependencies":
		return classDependencies
	case base == "paket.lock":
		return classLock
	case base == "paket.references":
		return classReferences
	}
	return ""
}

// Ecosystems: NuGet and the .NET base library are the C# plugin's islands (same ids
// and names, so the nodes merge); Paket's github/git/http dependencies are an
// island of their own.
//
// Implements: REQ-FSHARP-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return append(nuget.Ecosystems(), lang.Ecosystem{ID: nuget.Paket, Name: "Paket git, GitHub and HTTP sources"})
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Implements: REQ-FSHARP-002, REQ-FSHARP-003, REQ-FSHARP-005, REQ-FSHARP-006, REQ-FSHARP-007
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	switch class(f.Path) {
	case classProject:
		return extractProject(source), nil
	case classDependencies:
		return extractDependencies(source), nil
	case classLock:
		return extractLock(source), nil
	case classReferences:
		return extractReferences(source), nil
	}
	in := scanSource(string(source))
	return &lang.Extraction{Imports: in.imports, Symbols: in.symbols}, nil
}
