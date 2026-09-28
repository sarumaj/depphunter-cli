// Package dart analyzes Dart, Flutter included, and pub. A library's import, export
// and part directives (every URI of a configurable import) resolve to project files
// by relative URI, to a package's own lib/ by package: URI, to the lib/ of a local
// package the project depends on by path, shares a pub workspace or a melos
// repository with, to the hidden dart-std island (dart:) and flutter-sdk island (the
// packages Flutter's SDK ships), and to pub packages by what pubspec.yaml declares and
// pubspec.lock resolved (resolve.go). A pubspec's dependencies are imports of the
// packages they declare.
//
// The source is read by a scanner of its own (source.go, declarations.go), not the
// tree-sitter grammar, which was slow and failed on too many real Flutter files.
package dart

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemPub     = "pub"
	ecosystemStd     = "dart-std"
	ecosystemFlutter = "flutter-sdk"
)

// Implements: REQ-DART-001
type Plugin struct{}

func (Plugin) Name() string { return "dart" }
func (Plugin) Version() int { return 1 }

// Claims takes Dart sources and pubspec.yaml files, except what pub and the analyzer
// generate under .dart_tool.
//
// Implements: REQ-DART-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary || f.Path == ".dart_tool" || strings.HasPrefix(f.Path, ".dart_tool/") || strings.Contains(f.Path, "/.dart_tool/") {
		return false
	}
	return strings.EqualFold(path.Ext(f.Path), ".dart") || path.Base(f.Path) == "pubspec.yaml"
}

func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemPub, Name: "pub"},
		{ID: ecosystemStd, Name: "Dart SDK libraries", Std: true},
		{ID: ecosystemFlutter, Name: "Flutter SDK", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Extract reads a Dart library's directives and declarations, or a pubspec's
// dependencies and workspace members.
//
// Implements: REQ-DART-002, REQ-DART-003, REQ-DART-006
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	if !strings.EqualFold(path.Ext(f.Path), ".dart") {
		return extractPubspec(source)
	}
	extraction := &lang.Extraction{}
	tokens := tokenize(source)
	imports, i := directives(tokens)
	extraction.Imports = imports
	var symbols lang.SymbolSet
	declarations(tokens, i, &symbols)
	extraction.Symbols = symbols.List()
	return extraction, nil
}

// extractPubspec turns a pubspec's dependencies into imports of what they declare
// (a Flutter plugin used only by native code has no import to show it otherwise),
// and its workspace members into imports of their pubspecs.
//
// Implements: REQ-DART-006
func extractPubspec(source []byte) (*lang.Extraction, error) {
	p, err := readPubspec(source)
	if err != nil {
		return &lang.Extraction{}, nil // a broken pubspec imports nothing
	}
	extraction := &lang.Extraction{}
	for _, d := range p.dependencies {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: d.spec, Module: d.name, Name: kindDependency + ":" + d.section, Line: d.line})
	}
	for _, m := range p.workspace {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "workspace: " + m, Module: m, Name: kindMember, Line: p.memberLine[m]})
	}
	slices.SortStableFunc(extraction.Imports, func(a, b lang.RawImport) int { return cmp.Compare(a.Line, b.Line) })
	return extraction, nil
}
