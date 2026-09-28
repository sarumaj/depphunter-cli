// Package nim analyzes Nim modules (.nim), NimScript (.nims: config.nims and
// task scripts), nimble's package files (.nimble) and lock file (nimble.lock),
// Atlas's lock file (atlas.lock) and the compiler's nim.cfg files.
//
// An import resolves as the compiler finds a module: a path relative to the
// importing file's directory (./x and ../x only there); else a search root -
// the package's srcDir (from its .nimble) and the --path entries of the
// nim.cfg and config.nims files governing the file; std/x and the standard
// library's own module names are the standard library, or its files when the
// repository is Nim's own; pkg/x and any other name are a nimble package: the
// one whose installed files (nimbledeps/, Atlas's deps/, nimble.paths, the
// nimble directory) have the module, else the requirement the first segment
// names. A .nimble file's requirements are imports of their packages, pinned
// by nimble.lock or atlas.lock beside it (resolve.go).
//
// Nim is read by a lexer and an indentation-based scanner of its own (lex.go,
// source.go); the vendored tree-sitter grammar is not used (REQ-NIM-010).
package nim

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemNimble = "nimble"
	ecosystemStd    = "nim-std"
)

// The files told apart by name.
const (
	classNimble = "nimble:" // + the package file's name without .nimble
	classLock   = "nimble.lock"
	classAtlas  = "atlas.lock"
	classConfig = "cfg"
)

// Implements: REQ-NIM-001
type Plugin struct{}

func (Plugin) Name() string { return "nim" }
func (Plugin) Version() int { return 1 }

// Claims takes Nim modules, NimScript, .nimble files, nimble.lock, atlas.lock,
// nim.cfg and <name>.nim.cfg, except what nimble installed (nimbledeps/),
// Atlas cloned (deps/ beside a .nimble or an atlas.config) or the compiler
// generated (nimcache/).
//
// Implements: REQ-NIM-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary || class(f.Path) == "" && !nimSource(f.Path) {
		return false
	}
	return !ignored(f)
}

func nimSource(p string) bool {
	switch path.Ext(p) {
	case ".nim", ".nims":
		return true
	}
	return false
}

// Class tells the manifests apart; a .nimble file's class carries its name,
// which is the package's name unless it sets packageName.
//
// Implements: REQ-NIM-001
func (Plugin) Class(f *scan.File) string { return class(f.Path) }

func class(p string) string {
	base := path.Base(p)
	switch {
	case strings.HasSuffix(base, ".nimble") && len(base) > len(".nimble"):
		return classNimble + strings.TrimSuffix(base, ".nimble")
	case base == "nimble.lock":
		return classLock
	case base == "atlas.lock":
		return classAtlas
	case base == "nim.cfg", strings.HasSuffix(base, ".nim.cfg"):
		return classConfig
	}
	return ""
}

// ignored reports whether f lies in nimbledeps/ or nimcache/, or in a deps/
// directory beside a .nimble file or an Atlas configuration.
func ignored(f *scan.File) bool {
	segments := strings.Split(f.Path, "/")
	absolute := filepath.ToSlash(f.AbsolutePath)
	rooted := f.AbsolutePath != "" && strings.HasSuffix(absolute, f.Path)
	for i, s := range segments[:len(segments)-1] {
		switch s {
		case "nimbledeps", "nimcache":
			return true
		case "deps":
			if rooted && atlasDependencies(filepath.FromSlash(absolute[:len(absolute)-len(f.Path)]+strings.Join(segments[:i], "/"))) {
				return true
			}
		}
	}
	return false
}

var dependenciesMemo sync.Map // absolute directory -> bool: its deps/ is what Atlas cloned

// atlasDependencies reports whether dir's deps/ holds installed packages: directory has a
// .nimble file or an atlas.config, or deps/ has Atlas's atlas.config.
func atlasDependencies(directory string) bool {
	if v, ok := dependenciesMemo.Load(directory); ok {
		return v.(bool)
	}
	found := false
	if entries, err := os.ReadDir(directory); err == nil {
		for _, e := range entries {
			if n := e.Name(); strings.HasSuffix(n, ".nimble") || n == "atlas.config" || n == "atlas.workspace" {
				found = true
				break
			}
		}
	}
	if !found {
		_, err := os.Stat(filepath.Join(directory, "deps", "atlas.config"))
		found = err == nil
	}
	dependenciesMemo.Store(directory, found)
	return found
}

// Implements: REQ-NIM-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemNimble, Name: "Nimble packages"},
		{ID: ecosystemStd, Name: "Nim standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all, os.Getenv), nil
}

// Implements: REQ-NIM-002, REQ-NIM-003, REQ-NIM-004, REQ-NIM-005, REQ-NIM-006
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	switch c := class(f.Path); {
	case c == classLock:
		return extractLock(readNimbleLock(source), kindLock), nil
	case c == classAtlas:
		return extractLock(readAtlasLock(source), kindAtlas), nil
	case c == classConfig:
		extraction := &lang.Extraction{}
		addPaths(extraction, readConfig(source))
		return extraction, nil
	case strings.HasPrefix(c, classNimble):
		return extractNimble(source, strings.TrimPrefix(c, classNimble)), nil
	}
	s := scanSource(source)
	extraction := &lang.Extraction{Imports: s.imports, Symbols: s.symbols.List()}
	if path.Ext(f.Path) == ".nims" {
		addPaths(extraction, s.paths())
	}
	return extraction, nil
}

// extractNimble reads a .nimble file: its NimScript imports and symbols, the
// package as a symbol, each requirement as an import of its package, each bin
// as an edge to its main module, and each --path.
//
// Implements: REQ-NIM-005
func extractNimble(source []byte, base string) *lang.Extraction {
	s := scanSource(source)
	n := readNimble(source, base)
	extraction := &lang.Extraction{Imports: s.imports}
	var symbols lang.SymbolSet
	symbols.Add(n.name, "package", 1)
	for _, symbol := range s.symbols.List() {
		symbols.Add(symbol.Name, symbol.Kind, symbol.Line)
	}
	extraction.Symbols = symbols.List()
	seen := map[string]bool{}
	for _, r := range s.requirements() {
		// Old nimble files list several requirements in one string.
		for _, part := range strings.Split(r.text, ",") {
			part = strings.TrimSpace(part)
			spec := part
			if r.task != "" {
				spec = r.task + ": " + part
			}
			if seen[spec] || part == "" {
				continue
			}
			seen[spec] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: part, Name: kindRequire, Line: r.line})
		}
	}
	for _, b := range n.bins {
		if spec := "bin: " + b; !seen[spec] {
			seen[spec] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: b, Name: kindBin, Line: n.binLine})
		}
	}
	addPaths(extraction, s.paths())
	return extraction
}

func addPaths(extraction *lang.Extraction, paths []pathSwitch) {
	seen := map[string]bool{}
	for _, p := range paths {
		spec := "--path:" + p.value
		if !seen[spec] {
			seen[spec] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: p.value, Name: kindPath, Line: p.line})
		}
	}
}

// extractLock makes each package of a lock file an import, so what is locked
// is on the map even when no module imports it.
//
// Implements: REQ-NIM-006
func extractLock(packages []*locked, kind string) *lang.Extraction {
	extraction := &lang.Extraction{}
	for _, p := range packages {
		if strings.EqualFold(p.name, "nim") {
			continue
		}
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: p.name, Module: p.name, Name: kind, Line: p.line})
	}
	return extraction
}
