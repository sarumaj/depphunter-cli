// Package d analyzes D modules (.d, .di) and the dub package manager's files:
// dub.json, dub.sdl and dub.selections.json.
//
// `import a.b.c` names a module, which the compiler finds as a/b/c.d (or
// a/b/c/package.d) under an import directory: dub's importPaths and sourcePaths
// of the importing file's package, source/ or src/ by default, and those of the
// repository's packages it depends on. Modules of druntime and Phobos (core.*,
// std.*, etc.c.*, object) are the hidden standard library. Other modules belong
// to a dub package: the one dub fetched onto this machine that has the module,
// else a declared package the module's leading segments spell or a curated
// table names (resolve.go, known.go). `import("file")` reads a file under the
// string import directories (views/ by default).
//
// D is read by a small lexer of its own (lex.go), not the vendored tree-sitter
// grammar (REQ-DLANG-010).
package d

import (
	"path"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemDub = "dub"
	ecosystemStd = "d-std"
)

const (
	classJSON       = "json"
	classSDL        = "sdl"
	classSelections = "selections"
)

// Implements: REQ-DLANG-001
type Plugin struct{}

func (Plugin) Name() string { return "d" }
func (Plugin) Version() int { return 1 }

// Claims takes D modules - a .d file only when scan labeled it D, since make
// dependency files and DTrace scripts are .d files too - and dub's recipes and
// selections, except what lies in dub's .dub/ directory.
//
// Implements: REQ-DLANG-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary || dubDirectory(f.Path) {
		return false
	}
	switch path.Ext(f.Path) {
	case ".d":
		return f.Language == "D"
	case ".di":
		return true
	}
	return class(f.Path) != ""
}

// Class tells dub's files apart from other JSON files.
//
// Implements: REQ-DLANG-001
func (Plugin) Class(f *scan.File) string { return class(f.Path) }

func class(p string) string {
	switch path.Base(p) {
	case "dub.json":
		return classJSON
	case "dub.sdl":
		return classSDL
	case "dub.selections.json":
		return classSelections
	}
	return ""
}

// Implements: REQ-DLANG-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemDub, Name: "dub packages"},
		{ID: ecosystemStd, Name: "D runtime and standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-DLANG-002, REQ-DLANG-003, REQ-DLANG-005, REQ-DLANG-006
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	switch class(f.Path) {
	case classJSON:
		return extractRecipe(readJSONRecipe(source)), nil
	case classSDL:
		return extractRecipe(readSDLRecipe(source)), nil
	case classSelections:
		return extractSelections(source), nil
	}
	return extractSource(source), nil
}
