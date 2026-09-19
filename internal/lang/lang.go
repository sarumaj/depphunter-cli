// Package lang defines the contract every language ecosystem plugin implements
// (docs/REQUIREMENTS.md §4).
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

// Target is what an import resolves to. Exactly one of Local or Package is set.
type Target struct {
	Local     string // relative path of a project file or directory
	Ecosystem string // ecosystem id, e.g. "go", "go-std", "npm"
	Package   string // package / module name within the ecosystem
	Version   string
	// Unresolved means the package's owning module is unknown (missing from manifests).
	Unresolved bool
}

type Import struct {
	Spec   string // specifier as written in source
	Line   int
	Target Target
}

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
	// Claims reports whether the plugin analyses this file.
	Claims(f *scan.File) bool
	Ecosystems() []Ecosystem
	// Analyze receives all project files (so it can find manifests) and the claimed
	// subset, and returns results keyed by relative path.
	Analyze(ctx context.Context, root string, all, claimed []*scan.File) (map[string]*FileResult, error)
}
