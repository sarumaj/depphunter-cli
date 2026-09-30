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

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemHaxelib = "haxelib"
	ecosystemStd     = "haxe-std"
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
	if f.Binary || haxelibDirectory(f.Path) {
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

// haxelibDirectory reports whether a path lies in a local haxelib repository.
func haxelibDirectory(p string) bool {
	return strings.HasPrefix(p, ".haxelib/") || strings.Contains(p, "/.haxelib/")
}

var sniffed lang.Memo[string, bool] // absolute path and size -> a Lime project file

// sniffProject reads the head of a project.xml to tell a Lime/OpenFL project
// from another tool's file of that name.
func sniffProject(f *scan.File) bool {
	if f.TooLarge || f.Size > lang.MaxParseSize {
		return false
	}
	return sniffed.Get(fmt.Sprintf("%s\x00%d", f.AbsolutePath, f.Size), func(string) bool {
		source, err := os.ReadFile(f.AbsolutePath)
		return err == nil && limeProject(source)
	})
}

// Implements: REQ-HAXE-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemHaxelib, Name: "haxelib libraries"},
		{ID: ecosystemStd, Name: "Haxe standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all, os.Getenv), nil
}

// Implements: REQ-HAXE-002, REQ-HAXE-003, REQ-HAXE-005, REQ-HAXE-006
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	switch class(f.Path) {
	case classHXML:
		return extractHXML(source), nil
	case classLix:
		return extractLix(strings.TrimSuffix(path.Base(f.Path), ".hxml"), source), nil
	case classHaxelib:
		return extractHaxelib(source), nil
	case classProject:
		return extractProject(source), nil
	}
	return extractSource(source), nil
}
