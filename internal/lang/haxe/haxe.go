// Package haxe analyzes Haxe modules (.hx), the compiler's .hxml build files,
// haxelib's haxelib.json, lix's haxe_libraries/<name>.hxml and Lime/OpenFL
// project files (Project.xml, include.xml).
//
// A Haxe module is a file: `import a.b.C` names the module a/b/C.hx under a
// class path, `import a.b.C.D` the type D in it, `import a.b.*` every module
// of package a.b. The resolver indexes every module of the repository by the
// package it declares, so imports resolve whatever the class paths; other
// modules are Haxe's standard library (haxe.*, sys.*, the targets' packages
// and the top-level types) or belong to a haxelib library: the one installed
// (haxelib's repositories, lix's cache) that has the module, else a declared
// library its leading segments spell or a curated table names.
//
// Haxe is read by a small lexer of its own (lex.go), not the vendored
// tree-sitter grammar (REQ-HAXE-010).
package haxe

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoHaxelib = "haxelib"
	ecoStd     = "haxe-std"
)

const (
	classSource  = "source"
	classHXML    = "hxml"
	classLix     = "lix"
	classHaxelib = "haxelib"
	classProject = "project"
)

// Implements: REQ-HAXE-001
type Plugin struct{}

func (Plugin) Name() string { return "haxe" }
func (Plugin) Version() int { return 2 }

// Claims takes Haxe modules, .hxml files, haxelib.json and Lime/OpenFL project
// files - a project.xml only when its content says so - except what lies in a
// haxelib repository (.haxelib/).
//
// Implements: REQ-HAXE-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary || haxelibDir(f.Path) {
		return false
	}
	switch class(f.Path) {
	case "":
		return false
	case classProject:
		return sniffProject(f)
	}
	return true
}

// Class tells the kinds of files apart: an .hxml in haxe_libraries/ is a lix
// pin, not a build.
//
// Implements: REQ-HAXE-001
func (Plugin) Class(f *scan.File) string {
	c := class(f.Path)
	if c == classLix {
		return c + ":" + path.Base(f.Path) // the file's name is the library's
	}
	return c
}

func class(p string) string {
	base := path.Base(p)
	switch {
	case strings.HasSuffix(base, ".hx"):
		return classSource
	case strings.HasSuffix(base, ".hxml"):
		if path.Base(path.Dir(p)) == "haxe_libraries" {
			return classLix
		}
		return classHXML
	case base == "haxelib.json":
		return classHaxelib
	case base == "Project.xml" || base == "project.xml" || base == "include.xml":
		return classProject
	}
	return ""
}

// haxelibDir reports whether a path lies in a local haxelib repository.
func haxelibDir(p string) bool {
	return strings.HasPrefix(p, ".haxelib/") || strings.Contains(p, "/.haxelib/")
}

var sniffed sync.Map // absolute path and size -> bool: a Lime project file

// sniffProject reads the head of a project.xml to tell a Lime/OpenFL project
// from another tool's file of that name.
func sniffProject(f *scan.File) bool {
	if f.TooLarge || f.Size > lang.MaxParseSize {
		return false
	}
	key := fmt.Sprintf("%s\x00%d", f.Abs, f.Size)
	if v, ok := sniffed.Load(key); ok {
		return v.(bool)
	}
	src, err := os.ReadFile(f.Abs)
	ok := err == nil && limeProject(src)
	sniffed.Store(key, ok)
	return ok
}

// Implements: REQ-HAXE-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoHaxelib, Name: "haxelib libraries"},
		{ID: ecoStd, Name: "Haxe standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all, os.Getenv), nil
}

// Implements: REQ-HAXE-002, REQ-HAXE-003, REQ-HAXE-005, REQ-HAXE-006
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	switch class(f.Path) {
	case classHXML:
		return extractHXML(src), nil
	case classLix:
		return extractLix(strings.TrimSuffix(path.Base(f.Path), ".hxml"), src), nil
	case classHaxelib:
		return extractHaxelib(src), nil
	case classProject:
		return extractProject(src), nil
	}
	return extractSource(src), nil
}
