// Package markdown reads a repository's documentation as what it is: a set of
// dependencies. A README that links to docs/REQUIREMENTS.md depends on that file, and
// one that links to internal/server/server.go depends on that; both break when the
// target is moved, and nothing a package manager reads will say so.
//
// So the links are put on the map beside the imports. A link to a file or a directory
// in the repository becomes an edge from the document to it, and the headings become
// the file's symbols, so a document expands into its sections the way a source file
// expands into its functions.
//
// A link that leads nowhere becomes a finding instead (internal/findings): the file is
// not there, the heading it names is not in it, or the reference it uses was never
// defined. That check is exact and needs no network. An http(s) link is checked only
// with --online, and then conservatively - see the Web checker.
package markdown

import (
	"path"
	"strconv"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type Plugin struct{}

func (Plugin) Name() string { return "markdown" }
func (Plugin) Version() int { return 1 }

// Claims takes the documentation a repository carries. Extensions rather than names,
// since a repository's docs are not only its README.
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	switch path.Ext(f.Path) {
	case ".md", ".markdown", ".mdx":
		return true
	}
	return false
}

// Ecosystems is empty: a link is not a package. What a document depends on is inside
// the repository, and an island of external hosts would be a legend of somebody
// else's domain names rather than a dependency the map can say anything about.
func (Plugin) Ecosystems() []lang.Ecosystem { return nil }

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	ex := &lang.Extraction{}
	for _, l := range Links(src) {
		if l.Dest == "" {
			continue // a reference with no definition: nothing to point at
		}
		ex.Imports = append(ex.Imports, lang.RawImport{
			Spec: trim(l.Spec, 120), Module: l.Dest, Line: l.Line,
		})
	}
	// Headings are the document's symbols. Two sections may be called the same thing,
	// and a symbol's name is what identifies it within its file, so a repeat is
	// numbered the way a renderer numbers its anchor.
	seen := map[string]int{}
	for _, h := range Headings(src) {
		name := plain(h.Text)
		if name == "" {
			continue
		}
		if n := seen[name]; n > 0 {
			name += " (" + strconv.Itoa(n+1) + ")"
		}
		seen[plain(h.Text)]++
		ex.Symbols = append(ex.Symbols, lang.Symbol{Name: name, Kind: heading(h.Level), Line: h.Line})
	}
	return ex, nil
}

// heading names a level the way the side panel can show it.
func heading(level int) string {
	if level <= 1 {
		return "title"
	}
	return "heading " + strconv.Itoa(level)
}

// trim keeps a link short enough to read in a side panel.
func trim(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
