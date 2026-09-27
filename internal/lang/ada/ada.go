// Package ada analyzes Ada (and SPARK, which is Ada) sources (.ads specs,
// .adb bodies, .ada files of GNAT's other naming schemes), GNAT project files
// (.gpr) and Alire's crate manifest (alire.toml).
//
// `with A.B;` names a library unit, which the resolver finds by the unit names
// every source of the repository declares (resolve.go), so GNAT's file naming
// convention, krunched names and a project's Naming package need not be
// followed; a body is linked to its spec, a child unit to its parent and a
// subunit (separate (P)) to its parent body. Predefined units (Ada.*, System.*,
// Interfaces.*, GNAT.*, Standard and the Ada 83 renamings) are the hidden
// ada-std island. Other units belong to an Alire crate: the one Alire fetched
// (into alire/cache/, or Alire 2's shared cache) that has the unit or its
// parent, else a declared crate the unit's name spells or a curated table
// names (known.go), else an unresolved crate named by the unit's first
// segment.
//
// Ada is read by a small scanner of its own (lex.go, source.go), not the
// vendored tree-sitter grammar (REQ-ADA-010): the grammar parses well but
// takes 5 to 8 ms per file, forty times the scanner, and the resolver reads
// every source's unit header besides.
package ada

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
	ecoAlire = "alire"
	ecoStd   = "ada-std"
)

const classManifest = "alire"

func source(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".ads", ".adb", ".ada":
		return true
	}
	return false
}

func gprFile(p string) bool { return strings.EqualFold(path.Ext(p), ".gpr") }

// Implements: REQ-ADA-001
type Plugin struct{}

func (Plugin) Name() string { return "ada" }
func (Plugin) Version() int { return 1 }

// Claims takes Ada sources, project files and alire.toml, except what Alire
// keeps in the alire/ directory beside an alire.toml (its lock file, build
// cache and the crates it fetched) and what a build wrote into an obj/
// directory beside a project file (the binder's b__main.adb).
//
// Implements: REQ-ADA-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	if !source(f.Path) && !gprFile(f.Path) && path.Base(f.Path) != "alire.toml" {
		return false
	}
	return !generated(f)
}

// generated reports whether f lies in an alire/ directory beside an
// alire.toml or an obj/ directory beside a .gpr.
func generated(f *scan.File) bool {
	segments := strings.Split(f.Path, "/")
	for i, s := range segments[:len(segments)-1] {
		if s != "alire" && s != "obj" {
			continue
		}
		if f.Abs == "" || !strings.HasSuffix(filepath.ToSlash(f.Abs), f.Path) {
			continue
		}
		base := f.Abs[:len(f.Abs)-len(f.Path)]
		dir := filepath.Join(base, filepath.FromSlash(strings.Join(segments[:i], "/")))
		if s == "alire" && hasFile(dir, "alire.toml") || s == "obj" && hasFile(dir, "*.gpr") {
			return true
		}
	}
	return false
}

var dirFiles sync.Map // absolute directory + "\x00" + pattern -> bool

// hasFile reports whether dir has a file matching pattern (a name, or *.gpr).
func hasFile(dir, pattern string) bool {
	key := dir + "\x00" + pattern
	if v, ok := dirFiles.Load(key); ok {
		return v.(bool)
	}
	found := false
	if strings.HasPrefix(pattern, "*") {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.EqualFold(path.Ext(e.Name()), pattern[1:]) {
					found = true
					break
				}
			}
		}
	} else if _, err := os.Stat(filepath.Join(dir, pattern)); err == nil {
		found = true
	}
	dirFiles.Store(key, found)
	return found
}

// Class tells alire.toml apart from other TOML files.
//
// Implements: REQ-ADA-001
func (Plugin) Class(f *scan.File) string {
	if path.Base(f.Path) == "alire.toml" {
		return classManifest
	}
	return ""
}

// Implements: REQ-ADA-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoAlire, Name: "Alire crates"},
		{ID: ecoStd, Name: "Ada predefined units", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-ADA-002, REQ-ADA-003, REQ-ADA-005, REQ-ADA-006
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	switch {
	case path.Base(f.Path) == "alire.toml":
		return extractManifest(src), nil
	case gprFile(f.Path):
		return extractGPR(src), nil
	}
	s := extractSource(src)
	return &lang.Extraction{Imports: s.imports, Symbols: s.symbols.List()}, nil
}
