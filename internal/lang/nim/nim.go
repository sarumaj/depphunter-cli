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
	ecoNimble = "nimble"
	ecoStd    = "nim-std"
)

// The files told apart by name.
const (
	classNimble = "nimble:" // + the package file's name without .nimble
	classLock   = "nimble.lock"
	classAtlas  = "atlas.lock"
	classCfg    = "cfg"
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
		return classCfg
	}
	return ""
}

// ignored reports whether f lies in nimbledeps/ or nimcache/, or in a deps/
// directory beside a .nimble file or an Atlas configuration.
func ignored(f *scan.File) bool {
	segments := strings.Split(f.Path, "/")
	abs := filepath.ToSlash(f.Abs)
	rooted := f.Abs != "" && strings.HasSuffix(abs, f.Path)
	for i, s := range segments[:len(segments)-1] {
		switch s {
		case "nimbledeps", "nimcache":
			return true
		case "deps":
			if rooted && atlasDeps(filepath.FromSlash(abs[:len(abs)-len(f.Path)]+strings.Join(segments[:i], "/"))) {
				return true
			}
		}
	}
	return false
}

var depsMemo sync.Map // absolute directory -> bool: its deps/ is what Atlas cloned

// atlasDeps reports whether dir's deps/ holds installed packages: dir has a
// .nimble file or an atlas.config, or deps/ has Atlas's atlas.config.
func atlasDeps(dir string) bool {
	if v, ok := depsMemo.Load(dir); ok {
		return v.(bool)
	}
	found := false
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if n := e.Name(); strings.HasSuffix(n, ".nimble") || n == "atlas.config" || n == "atlas.workspace" {
				found = true
				break
			}
		}
	}
	if !found {
		_, err := os.Stat(filepath.Join(dir, "deps", "atlas.config"))
		found = err == nil
	}
	depsMemo.Store(dir, found)
	return found
}

// Implements: REQ-NIM-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoNimble, Name: "Nimble packages"},
		{ID: ecoStd, Name: "Nim standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all, os.Getenv), nil
}

// Implements: REQ-NIM-002, REQ-NIM-003, REQ-NIM-004, REQ-NIM-005, REQ-NIM-006
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	switch c := class(f.Path); {
	case c == classLock:
		return extractLock(readNimbleLock(src), kindLock), nil
	case c == classAtlas:
		return extractLock(readAtlasLock(src), kindAtlas), nil
	case c == classCfg:
		ex := &lang.Extraction{}
		addPaths(ex, readCfg(src))
		return ex, nil
	case strings.HasPrefix(c, classNimble):
		return extractNimble(src, strings.TrimPrefix(c, classNimble)), nil
	}
	s := scanSource(src)
	ex := &lang.Extraction{Imports: s.imports, Symbols: s.symbols.List()}
	if path.Ext(f.Path) == ".nims" {
		addPaths(ex, s.paths())
	}
	return ex, nil
}

// extractNimble reads a .nimble file: its NimScript imports and symbols, the
// package as a symbol, each requirement as an import of its package, each bin
// as an edge to its main module, and each --path.
//
// Implements: REQ-NIM-005
func extractNimble(src []byte, base string) *lang.Extraction {
	s := scanSource(src)
	n := readNimble(src, base)
	ex := &lang.Extraction{Imports: s.imports}
	var symbols lang.SymbolSet
	symbols.Add(n.name, "package", 1)
	for _, sym := range s.symbols.List() {
		symbols.Add(sym.Name, sym.Kind, sym.Line)
	}
	ex.Symbols = symbols.List()
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
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: part, Name: kindRequire, Line: r.line})
		}
	}
	for _, b := range n.bins {
		if spec := "bin: " + b; !seen[spec] {
			seen[spec] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: b, Name: kindBin, Line: n.binLine})
		}
	}
	addPaths(ex, s.paths())
	return ex
}

func addPaths(ex *lang.Extraction, paths []pathSwitch) {
	seen := map[string]bool{}
	for _, p := range paths {
		spec := "--path:" + p.value
		if !seen[spec] {
			seen[spec] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: p.value, Name: kindPath, Line: p.line})
		}
	}
}

// extractLock makes each package of a lock file an import, so what is locked
// is on the map even when no module imports it.
//
// Implements: REQ-NIM-006
func extractLock(pkgs []*locked, kind string) *lang.Extraction {
	ex := &lang.Extraction{}
	for _, p := range pkgs {
		if strings.EqualFold(p.name, "nim") {
			continue
		}
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: p.name, Module: p.name, Name: kind, Line: p.line})
	}
	return ex
}
