// Package graph defines the document exchanged between analysis and the UI.
// Its JSON shape is specified in docs/requirements/mod.
//
// The VS Code extension needs the same shape in TypeScript, and that file is
// generated from these declarations rather than written beside them: see
// types_test.go, which also checks on every ordinary test run that what is
// committed is still what this file says.
package graph

//go:generate go test . -update

import (
	"iter"
	"strings"
	"time"
)

// Implements: REQ-MOD-002
type NodeKind string

const (
	KindDirectory NodeKind = "dir"
	KindFile      NodeKind = "file"
	KindSymbol    NodeKind = "symbol"
	KindEcosystem NodeKind = "ecosystem"
	KindPackage   NodeKind = "package"
)

// Implements: REQ-MOD-006
type EdgeKind string

const (
	EdgeImport EdgeKind = "import"
	// EdgeReference links a symbol (or file) to a definition it uses, found by a
	// language server.
	//
	// Implements: REQ-MOD-007
	EdgeReference EdgeKind = "reference"
	// EdgeDepends links an external package to a package it depends on, read from the
	// project's lock files (see --resolve-depth).
	//
	// Implements: REQ-MOD-010
	EdgeDepends EdgeKind = "depends"
)

// Implements: REQ-MOD-004
type Node struct {
	ID       string   `json:"id"`
	Kind     NodeKind `json:"kind"`
	Name     string   `json:"name"`
	Path     string   `json:"path,omitempty"`
	Parent   string   `json:"parent,omitempty"`
	Language string   `json:"lang,omitempty"`
	LOC      int      `json:"loc,omitempty"`
	// Bytes is what the file measured on disk. LOC is what the map is built out of,
	// but a file can have no lines to count and still take up room: anything binary,
	// and anything over --max-file-size, is listed without ever being read. Sized by
	// lines alone those came out as flat slabs - a 4 MB model indistinguishable from
	// an empty file - so the bytes travel too, and the UI falls back to them.
	//
	// Implements: REQ-MOD-012
	Bytes      int64  `json:"bytes,omitempty"`
	SymbolKind string `json:"symbolKind,omitempty"`
	Line       int    `json:"line,omitempty"`
	// Implements: REQ-MOD-005
	Version string `json:"version,omitempty"`
	// Requested is the specifier a manifest asked for when a lock file pinned it to
	// another version, e.g. "^4.2.0" for version 4.3.1.
	//
	// Implements: REQ-MOD-005
	Requested string `json:"requested,omitempty"`
	// Floating marks an external package that is not fixed to one version: it will
	// resolve to something else once it is installed again.
	//
	// Implements: REQ-MOD-005
	Floating bool `json:"floating,omitempty"`
	// Transitive marks a package no file in the project imports: it is on the map
	// because something the project depends on depends on it.
	//
	// Implements: REQ-MOD-009
	Transitive bool `json:"transitive,omitempty"`
	// Index is the package index or mirror the package resolves from, and
	// IndexUnknown marks one that only the repository's own configuration names -
	// nothing on this machine vouches for it.
	Index        string `json:"index,omitempty"`
	IndexUnknown bool   `json:"indexUnknown,omitempty"`
	// Private marks a package this organization owns (--private, GOPRIVATE). Nothing
	// so marked is named to a public index or sent to the vulnerability database: the
	// request would be the disclosure.
	//
	// Implements: REQ-MOD-011
	Private bool `json:"private,omitempty"`
	// Std marks ecosystems holding a language's standard library, which the UI hides by default.
	Std bool `json:"std,omitempty"`
	// Unresolved marks packages whose owning module could not be determined from manifests.
	Unresolved bool `json:"unresolved,omitempty"`
	// Origin is where a package no index has was installed from: a local directory,
	// an archive or a VCS URL. Such a package is also Private.
	//
	// Implements: REQ-PY-015
	Origin string `json:"origin,omitempty"`
	// Git is the git checkout a package was built from, "<repository URL>#<full
	// commit>", when its Version shows something else (lang.Target.Git).
	//
	// Implements: REQ-FND-026
	Git string `json:"git,omitempty"`
	// Platform is the platforms a package installs on where it installs on some
	// only, "os=linux & cpu=x64" (lang.Target.Platform): one of the binaries a
	// package ships per platform, a dependency of the platform it names rather
	// than of every install.
	//
	// Implements: REQ-JS-018
	Platform string `json:"platform,omitempty"`
}

// Implements: REQ-MOD-006
type Edge struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Kind EdgeKind `json:"kind"`
	Line int      `json:"line,omitempty"`
}

// Implements: REQ-MOD-001
type Graph struct {
	Root        string    `json:"root"`
	GeneratedAt time.Time `json:"generatedAt"`
	Nodes       []*Node   `json:"nodes"`
	Edges       []*Edge   `json:"edges"`
}

// Of yields the graph's nodes of one kind, in the graph's order.
func (g *Graph) Of(kind NodeKind) iter.Seq[*Node] {
	return func(yield func(*Node) bool) {
		for _, n := range g.Nodes {
			if n.Kind == kind && !yield(n) {
				return
			}
		}
	}
}

// Implements: REQ-MOD-003
func DirectoryID(path string) string          { return "d:" + path }
func FileID(path string) string               { return "f:" + path }
func SymbolID(file, name string) string       { return "s:" + file + "#" + name }
func EcosystemID(ecosystem string) string     { return "e:" + ecosystem }
func PackageID(ecosystem, name string) string { return "p:" + ecosystem + ":" + name }

// EcosystemOf is the ecosystem an ecosystem ID names, the inverse of EcosystemID:
// "go" for "e:go". It is "" for any other ID.
//
// Implements: REQ-MOD-003
func EcosystemOf(id string) string { return after(id, "e:") }

// FileOf is the path a file ID names, the inverse of FileID: "src/main.go" for
// "f:src/main.go". It is "" for any other ID.
//
// Implements: REQ-MOD-003
func FileOf(id string) string { return after(id, "f:") }

// after is what follows prefix in id, or "" when id does not start with it.
func after(id, prefix string) string {
	if rest, ok := strings.CutPrefix(id, prefix); ok {
		return rest
	}
	return ""
}
