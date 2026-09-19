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

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type Symbol struct {
	Name string // unique within its file, e.g. "Server.Start" for methods
	Kind string // func, method, type, const, var, class, …
	Line int
}

// RawImport is an import as written in source.
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
type Target struct {
	Local     string // relative path of a project file or directory
	Ecosystem string // ecosystem id, e.g. "go", "go-std", "npm"
	Package   string // package / module name within the ecosystem
	Version   string
	// Unresolved means the package's owning module is unknown (missing from manifests).
	Unresolved bool
}

type Resolver interface {
	Resolve(file string, imp RawImport) Target
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

type Ecosystem struct {
	ID   string
	Name string
	Std  bool // standard library; hidden in the UI unless enabled
}

type Plugin interface {
	Name() string
	// Version must change whenever Extract's output for the same input changes,
	// so cached extractions from older builds are not reused.
	Version() int
	// Claims reports whether the plugin analyses this file.
	Claims(f *scan.File) bool
	Ecosystems() []Ecosystem
	// Extract must depend only on src and the file's extension.
	Extract(f *scan.File, src []byte) (*Extraction, error)
	// Resolver prepares import resolution from all project files (manifests, layout).
	Resolver(root string, all []*scan.File) (Resolver, error)
}

// Apply resolves an extraction's imports.
func Apply(r Resolver, file string, ex *Extraction) *FileResult {
	res := &FileResult{Symbols: ex.Symbols}
	for _, im := range ex.Imports {
		res.Imports = append(res.Imports, Import{Spec: im.Spec, Line: im.Line, Target: r.Resolve(file, im)})
	}
	return res
}

// Claimed returns the files p analyses.
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
