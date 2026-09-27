// Package objc analyzes Objective-C and Objective-C++ (.m, .mm, and the .h headers
// the scan finds to be Objective-C) and the manifests of CocoaPods and Carthage.
//
// `#import` and `#include` are read by the cpp plugin's preprocessor scanner and
// resolve as C includes do (project files, a compilation database, the standard
// and system headers), except that Apple's frameworks (<UIKit/UIKit.h>) go to the
// hidden apple-sdk island the swift plugin declares, and a framework header of a
// pod (<AFNetworking/AFNetworking.h>) or a module (`@import Firebase;`) goes to the
// pod the Podfile declares or Podfile.lock pins (internal/lang/cocoapods). A
// Podfile's `pod` lines, a podspec's dependencies and a Cartfile's entries are
// imports of the packages they name.
//
// Declarations are read by a hand-written scanner (lex.go, decls.go), not the
// vendored tree-sitter grammar: see REQ-OBJC-013.
package objc

import (
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cocoapods"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/lang/swift"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Import kinds, carried in RawImport.Name beside the cpp plugin's include kinds.
const (
	kindModule   = "module"   // @import Module;
	kindPod      = "pod"      // a Podfile's pod, a podspec's dependency
	kindCarthage = "carthage" // a Cartfile entry; Module is "carthage:<package>"
)

// Implements: REQ-OBJC-001
type Plugin struct{}

func (Plugin) Name() string { return "objc" }
func (Plugin) Version() int { return 1 }

// Claims takes Objective-C sources: ".mm", and ".m" and ".h" files the scan
// labelled Objective-C by their content (a ".m" may be MATLAB or Mercury, a ".h" a
// C or C++ header - those are left to others), plus the CocoaPods and Carthage
// manifests.
//
// Implements: REQ-OBJC-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	switch strings.ToLower(path.Ext(f.Path)) {
	case ".mm":
		return true
	case ".m", ".h":
		return f.Lang == "Objective-C"
	}
	return cocoapods.Kind(f.Path) != ""
}

// Class tells manifests (named, not by extension: Podfile, Cartfile, x.podspec.json)
// from sources.
//
// Implements: REQ-LANG-025
func (Plugin) Class(f *scan.File) string { return cocoapods.Kind(f.Path) }

func (Plugin) Ecosystems() []lang.Ecosystem {
	eco := append(cocoapods.Ecosystems(), lang.Ecosystem{ID: swift.AppleEcosystem, Name: "Apple SDKs", Std: true})
	return append(eco, cpp.Plugin{}.Ecosystems()...)
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// define matches a #define with a value or parameters: header guards have neither.
var define = regexp.MustCompile(`^[ \t]*#[ \t]*define[ \t]+([A-Za-z_]\w*)(?:\(|[ \t]+\S)`)

// Implements: REQ-OBJC-002, REQ-OBJC-003, REQ-OBJC-007, REQ-OBJC-010, REQ-OBJC-011
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	if kind := cocoapods.Kind(f.Path); kind != "" {
		ex := &lang.Extraction{}
		for _, d := range cocoapods.Deps(kind, src) {
			name := kindPod
			if kind == "cartfile" {
				name = kindCarthage
			}
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: d.Spec, Module: d.Name, Name: name, Line: d.Line})
		}
		return ex, nil
	}
	includes, dead := cpp.Preprocess(src)
	tokens := lex(src)
	live := tokens[:0:0]
	for _, t := range tokens {
		if t.line >= len(dead) || !dead[t.line] {
			live = append(live, t)
		}
	}
	p := parse(live)
	for i, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, "define") && (i+1 >= len(dead) || !dead[i+1]) {
			if m := define.FindStringSubmatch(line); m != nil {
				p.add(m[1], "macro", i+1, false)
			}
		}
	}
	imports := append(includes, p.modules...)
	slices.SortStableFunc(imports, func(a, b lang.RawImport) int { return a.Line - b.Line })
	return &lang.Extraction{Imports: imports, Symbols: symbols(p.defs)}, nil
}
