// Package lang defines the contract every language ecosystem plugin implements
// (docs/REQUIREMENTS.md §4).
//
// Analysis is split in two so parsing can be cached: Extract reads one file's
// content and nothing else (its result is cached by content hash), while a Resolver,
// built once per run from the whole file list and its manifests, maps raw imports to
// targets.
package lang

import (
	"context"
	"path"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Implements: REQ-LANG-003
type Symbol struct {
	Name string // unique within its file, e.g. "Server.Start" for methods
	Kind string // func, method, type, const, var, class, …
	Line int
}

// RawImport is an import as written in source.
//
// Implements: REQ-LANG-002
type RawImport struct {
	Spec   string // as shown to users, e.g. "from .models import User"
	Module string // what resolution looks up: import path, specifier, dotted module
	Name   string // plugin-specific qualifier, e.g. Python's "from Module import Name"
	Line   int
}

// Extraction is everything a plugin reads from one file.
type Extraction struct {
	Imports []RawImport
	Symbols []Symbol
}

// Target is what an import resolves to. Exactly one of Local or Package is set;
// neither means the import is dropped (e.g. it points outside the project).
//
// Implements: REQ-LANG-004
type Target struct {
	Local     string // relative path of a project file or directory
	Ecosystem string // ecosystem id, e.g. "go", "go-std", "npm"
	Package   string // package / module name within the ecosystem
	Version   string // the version in use: exact when Pinned, else what the manifest says
	// Requested is what the manifest asked for when a lock file resolved it to
	// something else: "^4.2.0" locked to 4.3.1.
	Requested string
	// Pinned marks a dependency fixed to one version - by a lock file, an exact
	// specifier or a digest - rather than one that moves when it is next installed
	// (see Pinned in version.go).
	Pinned bool
	// Floating marks a dependency that moves although it names no version at all: a
	// GitLab template served by the instance, a remote include, an unversioned
	// reference. Without it such a target would read as "nothing known".
	Floating bool
	// Unresolved means the package's owning module is unknown (missing from manifests).
	Unresolved bool
}

type Resolver interface {
	Resolve(file string, imp RawImport) Target
}

// Transitive is the optional half of a Resolver: what an external package itself
// depends on, as far as the project's own lock files say. It is how --resolve-depth
// reaches past the packages a project imports directly without asking a registry,
// which is why the answer is only as complete as the lock files are.
//
// Implements: REQ-SUP-011
type Transitive interface {
	// Dependencies lists what t depends on. Nothing known and nothing to declare look
	// alike here; both return no targets.
	Dependencies(t Target) []Target
}

type Import struct {
	Spec   string
	Line   int
	Target Target
}

// FileResult is a file's extraction with its imports resolved.
type FileResult struct {
	Symbols []Symbol
	Imports []Import
}

// Implements: REQ-LANG-005
type Ecosystem struct {
	ID   string
	Name string
	Std  bool // standard library; hidden in the UI unless enabled
}

// Implements: REQ-LANG-001, REQ-LANG-025
type Plugin interface {
	Name() string
	// Version must change whenever Extract's output for the same input changes,
	// so cached extractions from older builds are not reused.
	Version() int
	// Claims reports whether the plugin analyzes this file.
	Claims(f *scan.File) bool
	Ecosystems() []Ecosystem
	// Extract must depend only on src and the file's extension, or else the plugin
	// implements Classifier to say what else it depends on.
	Extract(f *scan.File, src []byte) (*Extraction, error)
	// Resolver prepares import resolution from all project files (manifests, layout).
	Resolver(root string, all []*scan.File) (Resolver, error)
}

// Classifier is implemented by a plugin whose Extract depends on more of the file
// than its extension - its directory, its name. Class names that, and becomes part of
// the cache key, so a cached extraction is not read back for the same content in a
// place that is read differently.
//
// Implements: REQ-LANG-025
type Classifier interface {
	Class(f *scan.File) string
}

// ClassOf is what, besides the content, a plugin's extraction of f depends on.
//
// Implements: REQ-LANG-026
func ClassOf(p Plugin, f *scan.File) string {
	class := path.Ext(f.Path)
	if c, ok := p.(Classifier); ok {
		class += "|" + c.Class(f)
	}
	return class
}

// Apply resolves an extraction's imports.
func Apply(r Resolver, file string, ex *Extraction) *FileResult {
	res := &FileResult{Symbols: ex.Symbols}
	for _, im := range ex.Imports {
		res.Imports = append(res.Imports, Import{Spec: im.Spec, Line: im.Line, Target: r.Resolve(file, im)})
	}
	return res
}

// Claimed returns the files p analyzes.
//
// Implements: REQ-LANG-001
func Claimed(p Plugin, all []*scan.File) []*scan.File {
	var out []*scan.File
	for _, f := range all {
		if p.Claims(f) {
			out = append(out, f)
		}
	}
	return out
}

// Analyze runs p over the project without caching; results are keyed by path.
func Analyze(ctx context.Context, p Plugin, root string, all []*scan.File) (map[string]*FileResult, error) {
	r, err := p.Resolver(root, all)
	if err != nil {
		return nil, err
	}
	return ForEachFile(ctx, Claimed(p, all), func(f *scan.File, src []byte) *FileResult {
		ex, err := p.Extract(f, src)
		if err != nil {
			return nil
		}
		return Apply(r, f.Path, ex)
	}), ctx.Err()
}
