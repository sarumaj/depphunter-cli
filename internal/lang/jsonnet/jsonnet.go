// Package jsonnet analyzes Jsonnet (.jsonnet, .libsonnet) and
// jsonnet-bundler's manifest and lock file (jsonnetfile.json,
// jsonnetfile.lock.json).
//
// An import, importstr or importbin resolves as jsonnet finds the file:
// relative to the importing file, then along the library path - the vendor/
// and lib/ directories of the jsonnet-bundler projects above it (nearest
// first) and JSONNET_PATH. A file jb installed under vendor/ belongs to the
// dependency that installed it, told by its full path
// (github.com/org/repo/subdir/...) or its legacy link (vendor/<name>); when
// nothing is installed the manifests name the package the same way. A
// jsonnetfile's dependencies are imports of their packages, pinned by the
// lock beside it (resolve.go).
//
// Jsonnet is read by a lexer of its own (lex.go, source.go); the vendored
// tree-sitter grammar is not used (REQ-JSONNET-009).
package jsonnet

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const ecosystemJB = "jsonnet-bundler"

// The manifests, told apart by name (both are .json).
const (
	classManifest = "jsonnetfile"
	classLock     = "lock"
)

// Implements: REQ-JSONNET-001
type Plugin struct{}

func (Plugin) Name() string { return "jsonnet" }
func (Plugin) Version() int { return 1 }

// Claims takes Jsonnet files, jsonnetfile.json and jsonnetfile.lock.json,
// except what jsonnet-bundler installed into vendor/ beside a
// jsonnetfile.json.
//
// Implements: REQ-JSONNET-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	switch path.Ext(f.Path) {
	case ".jsonnet", ".libsonnet":
	default:
		if class(f.Path) == "" {
			return false
		}
	}
	return !ignored(f)
}

// Implements: REQ-JSONNET-001
func (Plugin) Class(f *scan.File) string { return class(f.Path) }

func class(p string) string {
	switch path.Base(p) {
	case "jsonnetfile.json":
		return classManifest
	case "jsonnetfile.lock.json":
		return classLock
	}
	return ""
}

// ignored reports whether f lies in a vendor/ directory beside a
// jsonnetfile.json.
func ignored(f *scan.File) bool {
	segments := strings.Split(f.Path, "/")
	absolute := filepath.ToSlash(f.AbsolutePath)
	if f.AbsolutePath == "" || !strings.HasSuffix(absolute, f.Path) {
		return false
	}
	base := absolute[:len(absolute)-len(f.Path)]
	for i, s := range segments[:len(segments)-1] {
		if s == "vendor" && jbRoot(filepath.FromSlash(base+strings.Join(segments[:i], "/"))) {
			return true
		}
	}
	return false
}

var rootMemo sync.Map // absolute directory -> bool: it has a jsonnetfile.json

func jbRoot(directory string) bool {
	if v, ok := rootMemo.Load(directory); ok {
		return v.(bool)
	}
	// Lstat: a marker committed as a symbolic link says nothing of its target.
	_, err := os.Lstat(filepath.Join(directory, "jsonnetfile.json"))
	rootMemo.Store(directory, err == nil)
	return err == nil
}

// Implements: REQ-JSONNET-008
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{{ID: ecosystemJB, Name: "jsonnet-bundler packages"}}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all, os.Getenv), nil
}

// Implements: REQ-JSONNET-002, REQ-JSONNET-003, REQ-JSONNET-005
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	switch class(f.Path) {
	case classManifest:
		dependencies, _ := readJsonnetfile(source)
		return manifestImports(dependencies, kindDependency), nil
	case classLock:
		dependencies, _ := readJsonnetfile(source)
		return manifestImports(dependencies, kindLock), nil
	}
	return extractSource(source), nil
}

// manifestImports makes each dependency an import of its manifest, so what a
// project declares or locks is on the map even when no file imports it.
func manifestImports(dependencies []*dependency, kind string) *lang.Extraction {
	extraction := &lang.Extraction{}
	seen := map[string]bool{}
	for _, d := range dependencies {
		if p := d.packageName(); !seen[p] {
			seen[p] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: p, Module: p, Name: kind, Line: d.line})
		}
	}
	return extraction
}
