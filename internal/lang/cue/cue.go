// Package cue analyzes CUE (.cue) and its module file (cue.mod/module.cue).
//
// An import names a package: one of the module's own (its path under the
// module path of cue.mod/module.cue, a directory, and the package named by
// the import's :qualifier or last element - every file of it in that
// directory), a Go package `cue get go` generated into cue.mod/gen (the Go
// module go.mod requires, or the Go standard library), a package vendored
// the old way into cue.mod/pkg, a dependency module.cue declares (pinned by
// its exact version), or CUE's standard library (no dot in the first path
// element). module.cue's dependencies are imports of their modules
// (resolve.go).
//
// CUE is read by a lexer of its own (lex.go, source.go); the vendored
// tree-sitter grammar is not used (REQ-CUE-009).
package cue

import (
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoCUE    = "cue"
	ecoStd    = "cue-std"
	ecoGo     = "go"     // the golang plugin's island: Go packages cue.mod/gen was generated from
	ecoGoStd  = "go-std" // and the Go standard library's
	classMod  = "module"
	maxImport = 64 // files one import of a multi-file package links to
)

// Implements: REQ-CUE-001
type Plugin struct{}

func (Plugin) Name() string { return "cue" }
func (Plugin) Version() int { return 1 }

// Claims takes .cue files except the dependency trees under cue.mod
// (pkg/, gen/, usr/).
//
// Implements: REQ-CUE-001
func (Plugin) Claims(f *scan.File) bool {
	return !f.Binary && path.Ext(f.Path) == ".cue" && !ignored(f.Path)
}

// ignored reports whether p lies in cue.mod/pkg, cue.mod/gen or cue.mod/usr.
func ignored(p string) bool {
	segments := strings.Split(p, "/")
	for i := 0; i+2 < len(segments); i++ {
		if segments[i] == "cue.mod" {
			switch segments[i+1] {
			case "pkg", "gen", "usr":
				return true
			}
		}
	}
	return false
}

// Class tells the module file from other CUE files.
//
// Implements: REQ-CUE-001
func (Plugin) Class(f *scan.File) string { return class(f.Path) }

func class(p string) string {
	if p == "cue.mod/module.cue" || strings.HasSuffix(p, "/cue.mod/module.cue") {
		return classMod
	}
	return ""
}

// Implements: REQ-CUE-008
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoCUE, Name: "CUE modules"},
		{ID: ecoStd, Name: "CUE standard library", Std: true},
		{ID: ecoGo, Name: "Go modules"},
		{ID: ecoGoStd, Name: "Go standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-CUE-002, REQ-CUE-003, REQ-CUE-005
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	ex := extractSource(src)
	if class(f.Path) == classMod {
		for _, d := range readModule(src).deps {
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: d.key, Module: d.key, Name: kindDep, Line: d.line})
		}
	}
	return ex, nil
}

// std lists CUE's standard library packages.
var std = map[string]bool{
	"crypto/ed25519": true, "crypto/hmac": true, "crypto/md5": true, "crypto/sha1": true,
	"crypto/sha256": true, "crypto/sha512": true, "encoding/base64": true, "encoding/csv": true,
	"encoding/hex": true, "encoding/json": true, "encoding/toml": true, "encoding/yaml": true,
	"html": true, "list": true, "math": true, "math/bits": true, "net": true, "path": true,
	"regexp": true, "strconv": true, "strings": true, "struct": true, "text/tabwriter": true,
	"text/template": true, "time": true, "tool": true, "tool/cli": true, "tool/exec": true,
	"tool/file": true, "tool/http": true, "tool/os": true, "uuid": true,
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func readFile(abs string, limit int64) []byte {
	f, err := os.Open(abs)
	if err != nil {
		return nil
	}
	defer f.Close()
	b, _ := io.ReadAll(io.LimitReader(f, limit))
	return b
}
