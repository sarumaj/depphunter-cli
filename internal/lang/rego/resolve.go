package rego

import (
	"os"
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// maxFiles bounds the files one import is expanded to.
const maxFiles = 64

type resolver struct {
	packages map[string][]string // package path (a.b) -> the files declaring it, sorted
}

// newResolver indexes the repository's policies by the package each file
// declares.
func newResolver(all []*scan.File) *resolver {
	r := &resolver{packages: map[string][]string{}}
	for _, f := range all {
		if path.Ext(f.Path) != ".rego" || f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			continue
		}
		if p := packageOf(source); p != "" {
			r.packages[p] = append(r.packages[p], f.Path)
		}
	}
	for _, files := range r.packages {
		sort.Strings(files)
	}
	return r
}

// packageOf is the package a file declares.
func packageOf(source []byte) string {
	tokens := lex(source)
	for i, t := range tokens {
		if t.kind == tIdentifier && t.first && t.text == "package" {
			segments, _ := readReference(tokens, i+1)
			return strings.Join(segments, ".")
		}
	}
	return ""
}

// Expand links an import or a reference of data.a.b.c to every file (but the
// importer) of the longest package its path starts with, unless an import of
// the file already links that package. A reference no
// package declares is external data and vanishes; so does an import of a
// namespace whose packages lie below it (import data.lib, then lib.x.y).
//
// Implements: REQ-REGO-004
func (r *resolver) Expand(file string, rawImport lang.RawImport) ([]lang.Import, bool) {
	via, reference := strings.CutPrefix(rawImport.Name, kindReference)
	packageName, files := r.lookup(rawImport.Module)
	if files != nil {
		for _, v := range strings.Split(via, ",") {
			if p, _ := r.lookup(v); reference && v != "" && p == packageName {
				return nil, true // an import links that package already
			}
		}
		var out []lang.Import
		for _, f := range files {
			if f != file && len(out) < maxFiles {
				out = append(out, lang.Import{Line: rawImport.Line, Target: lang.Target{Local: f}})
			}
		}
		for i := range out {
			out[i].Spec = rawImport.Spec
			if len(out) > 1 {
				out[i].Spec += " (" + out[i].Target.Local + ")"
			}
		}
		return out, true
	}
	if reference {
		return nil, true
	}
	prefix := rawImport.Module + "."
	for p := range r.packages {
		if strings.HasPrefix(p, prefix) {
			return nil, true
		}
	}
	return nil, false
}

// lookup finds the longest package a data path starts with.
func (r *resolver) lookup(p string) (string, []string) {
	segments := strings.Split(p, ".")
	for k := len(segments); k >= 1; k-- {
		packageName := strings.Join(segments[:k], ".")
		if files := r.packages[packageName]; len(files) > 0 {
			return packageName, files
		}
	}
	return "", nil
}

// Resolve drops what Expand did not link: data no policy declares.
//
// Implements: REQ-REGO-004, REQ-REGO-007
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target { return lang.Target{} }
