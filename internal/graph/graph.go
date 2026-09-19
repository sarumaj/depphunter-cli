// Package graph defines the document exchanged between analysis and the UI.
// Its JSON shape is specified in docs/REQUIREMENTS.md §3.
package graph

import "time"

type NodeKind string

const (
	KindDir       NodeKind = "dir"
	KindFile      NodeKind = "file"
	KindSymbol    NodeKind = "symbol"
	KindEcosystem NodeKind = "ecosystem"
	KindPackage   NodeKind = "package"
)

type EdgeKind string

const EdgeImport EdgeKind = "import"

type Node struct {
	ID         string   `json:"id"`
	Kind       NodeKind `json:"kind"`
	Name       string   `json:"name"`
	Path       string   `json:"path,omitempty"`
	Parent     string   `json:"parent,omitempty"`
	Lang       string   `json:"lang,omitempty"`
	LOC        int      `json:"loc,omitempty"`
	SymbolKind string   `json:"symbolKind,omitempty"`
	Line       int      `json:"line,omitempty"`
	Version    string   `json:"version,omitempty"`
	// Std marks ecosystems holding a language's standard library, which the UI hides by default.
	Std bool `json:"std,omitempty"`
	// Unresolved marks packages whose owning module could not be determined from manifests.
	Unresolved bool `json:"unresolved,omitempty"`
}

type Edge struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Kind EdgeKind `json:"kind"`
	Line int      `json:"line,omitempty"`
}

type Graph struct {
	Root        string    `json:"root"`
	GeneratedAt time.Time `json:"generatedAt"`
	Nodes       []*Node   `json:"nodes"`
	Edges       []*Edge   `json:"edges"`
}

func DirID(path string) string          { return "d:" + path }
func FileID(path string) string         { return "f:" + path }
func SymbolID(file, name string) string { return "s:" + file + "#" + name }
func EcosystemID(eco string) string     { return "e:" + eco }
func PackageID(eco, name string) string { return "p:" + eco + ":" + name }
