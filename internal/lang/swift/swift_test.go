package swift

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// A SwiftPM package with a Package.resolved, a local path package, and an Xcode app
// beside it whose packages are in its project.pbxproj and the workspace's
// Package.resolved: modules of the package's own targets, of the toolchain and the
// SDKs, of declared packages by product, by table and by name, directories named
// after an Xcode project's modules, and types used across the files of a module.
//
// Verifies: REQ-SWIFT-001, REQ-SWIFT-002, REQ-SWIFT-003, REQ-SWIFT-004, REQ-SWIFT-005
// Verifies: REQ-SWIFT-006, REQ-SWIFT-007, REQ-SWIFT-008, REQ-SWIFT-009, REQ-SWIFT-010
// Verifies: REQ-SWIFT-011
func TestPackageAndApp(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	imports := map[string]map[string]lang.Target{
		"App/App/AppModel.swift": {
			`import Combine`:   {Ecosystem: ecosystemApple, Package: "Combine"},
			`MainActor`:        {},
			`ObservableObject`: {},
			`Published`:        {},
		},
		"App/App/ContentView.swift": {
			`import SwiftUI`:    {Ecosystem: ecosystemApple, Package: "SwiftUI"},
			`import Alamofire`:  {Ecosystem: ecosystemSwiftPM, Package: "github.com/Alamofire/Alamofire", Version: "5.8.1", Requested: "5.8.0..<6.0.0", Pinned: true},
			`import Kingfisher`: {Ecosystem: ecosystemSwiftPM, Package: "github.com/onevcat/Kingfisher", Version: "7.10.0", Pinned: true},
			`import CoreKit`:    {Local: "App/CoreKit"},
			`View`:              {},
			`AppModel`:          {Local: "App/App/AppModel.swift"},
			`StateObject`:       {},
			`CoreThing`:         {Local: "App/CoreKit/Core.swift"},
			`Text`:              {},
		},
		"App/AppTests/AppTests.swift": {
			`import XCTest`:        {Ecosystem: ecosystemStd, Package: "XCTest"},
			`@testable import App`: {Local: "App"},
			`import CoreKit`:       {Local: "App/CoreKit"},
			`XCTestCase`:           {},
			`AppModel`:             {Local: "App/App/AppModel.swift"},
			`CoreThing`:            {Local: "App/CoreKit/Core.swift"},
		},
		"App/CoreKit/Core.swift": {},
		"Local/LocalKit/Package.swift": {
			`import PackageDescription`: {Ecosystem: ecosystemStd, Package: "PackageDescription"},
			`Package`:                   {},
		},
		"Local/LocalKit/Sources/LocalKit/LocalKit.swift": {},
		"Package.swift": {
			`import PackageDescription`: {Ecosystem: ecosystemStd, Package: "PackageDescription"},
			`Package`:                   {},
			`.package(url: "https://github.com/apple/swift-nio.git", from: "2.60.0")`:                                    {Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-nio", Version: "2.64.0", Requested: "2.60.0..<3.0.0", Pinned: true},
			`.package(url: "https://github.com/apple/swift-log", .upToNextMajor(from: "1.5.0"))`:                         {Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-log", Version: "1.5.4", Requested: "1.5.0..<2.0.0", Pinned: true},
			`.package(url: "git@github.com:apple/swift-argument-parser.git", exact: "1.3.0")`:                            {Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-argument-parser", Version: "1.3.0", Pinned: true},
			`.package(url: "https://github.com/apple/swift-collections", "1.0.0"..<"2.0.0")`:                             {Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-collections", Version: "1.1.0", Requested: "1.0.0..<2.0.0", Pinned: true},
			`.package(url: "https://github.com/swiftlang/swift-markdown.git", branch: "main")`:                           {Ecosystem: ecosystemSwiftPM, Package: "github.com/swiftlang/swift-markdown", Version: "4aae40bf6fff5286e0e1672329d17824ce16e081", Requested: "branch main", Pinned: true},
			`.package(url: "https://github.com/acme/private-kit", revision: "0123456789abcdef0123456789abcdef01234567")`: {Ecosystem: ecosystemSwiftPM, Package: "github.com/acme/private-kit", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true},
			`.package(path: "Local/LocalKit")`:                                                                           {Local: "Local/LocalKit"},
			`.package(id: "mona.LinkedList", from: "1.0.0")`:                                                             {Ecosystem: ecosystemSwiftPM, Package: "mona.LinkedList", Version: "1.0.0..<2.0.0"},
		},
		"Sources/Demo/Model.swift": {
			`String`: {},
			`Void`:   {},
		},
		"Sources/Demo/Server.swift": {
			`import Foundation`:               {Ecosystem: ecosystemStd, Package: "Foundation"},
			`import NIOCore`:                  {Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-nio", Version: "2.64.0", Requested: "2.60.0..<3.0.0", Pinned: true},
			`@preconcurrency import Logging`:  {Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-log", Version: "1.5.4", Requested: "1.5.0..<2.0.0", Pinned: true},
			`@_exported import Util`:          {Local: "Sources/Util"},
			`import struct Collections.Deque`: {Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-collections", Version: "1.1.0", Requested: "1.0.0..<2.0.0", Pinned: true},
			`import Markdown`:                 {Ecosystem: ecosystemSwiftPM, Package: "github.com/swiftlang/swift-markdown", Version: "4aae40bf6fff5286e0e1672329d17824ce16e081", Requested: "branch main", Pinned: true},
			`import UIKit`:                    {Ecosystem: ecosystemApple, Package: "UIKit"},
			`import AppKit`:                   {Ecosystem: ecosystemApple, Package: "AppKit"},
			`Service`:                         {Local: "Sources/Demo/Model.swift"},
			`Logger`:                          {},
			`Deque`:                           {},
			`Job`:                             {Local: "Sources/Demo/Model.swift"},
			`Int`:                             {},
			`Helper`:                          {Local: "Sources/Util/Helper.swift"},
			`Status`:                          {Local: "Sources/Demo/Model.swift"},
			`CustomStringConvertible`:         {},
			`String`:                          {},
		},
		"Sources/Util/Helper.swift": {
			`import Collections`: {Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-collections", Version: "1.1.0", Requested: "1.0.0..<2.0.0", Pinned: true},
			`import LinkedList`:  {Ecosystem: ecosystemSwiftPM, Package: "mona.LinkedList", Version: "1.0.0..<2.0.0"},
			`Array`:              {},
			`Int`:                {},
		},
		"Sources/demo-cli/main.swift": {
			`import ArgumentParser`: {Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-argument-parser", Version: "1.3.0", Pinned: true},
			`import Demo`:           {Local: "Sources/Demo"},
			`import SnapKit`:        {Ecosystem: ecosystemSwiftPM, Package: "github.com/SnapKit/SnapKit", Unresolved: true},
			`import Unknown`:        {Ecosystem: ecosystemSwiftPM, Package: "Unknown", Unresolved: true},
			`Server`:                {Local: "Sources/Demo/Server.swift"},
		},
		"Tests/DemoTests/ServerTests.swift": {
			`import XCTest`:         {Ecosystem: ecosystemStd, Package: "XCTest"},
			`@testable import Demo`: {Local: "Sources/Demo"},
			`XCTestCase`:            {},
			`Server`:                {Local: "Sources/Demo/Server.swift"},
			`Status`:                {Local: "Sources/Demo/Model.swift"},
			`XCTAssertEqual`:        {},
		},
	}
	symbols := map[string]map[string]string{
		"App/App/AppModel.swift":                         {"AppModel": "class", "AppModel.count": "property"},
		"App/App/ContentView.swift":                      {"ContentView": "type", "ContentView.model": "property", "ContentView.body": "property"},
		"App/AppTests/AppTests.swift":                    {"AppTests": "class", "AppTests.testModel": "method"},
		"App/CoreKit/Core.swift":                         {"CoreThing": "type", "CoreThing.name": "property"},
		"Local/LocalKit/Package.swift":                   {"package": "var"},
		"Local/LocalKit/Sources/LocalKit/LocalKit.swift": {"LocalKit": "type"},
		"Package.swift":                                  {"package": "var"},
		"Sources/Demo/Model.swift":                       {"Service": "interface", "Service.start": "method", "Status": "type", "Status.label": "method", "Job": "type", "Job.Step": "type", "Hidden": "type", "Registry": "class", "Handler": "type"},
		"Sources/Demo/Server.swift":                      {"Server": "class", "Server.logger": "property", "Server.queue": "property", "Server.shared": "property", "Server.init": "method", "Server.start": "method", "Server.debugDump": "method", "Server@33": "extension", "Server.description": "property", "makeServer": "func", "defaultPort": "var"},
		"Sources/Util/Helper.swift":                      {"Helper": "type", "Helper.init": "method", "Array": "extension", "Array.sum": "method"},
		"Sources/demo-cli/main.swift":                    {"server": "var"},
		"Tests/DemoTests/ServerTests.swift":              {"ServerTests": "class", "ServerTests.testStart": "method"},
	}
	for file, r := range results {
		t.Run(file, func(t *testing.T) {
			langtest.CheckImports(t, r, imports[file])
			langtest.CheckSymbols(t, r, symbols[file])
		})
	}
	for file := range symbols {
		if results[file] == nil {
			t.Errorf("%s: not analyzed", file)
		}
	}
}

// A package SwiftPM checked out under .build/checkouts names its own dependencies;
// the project's Package.resolved pins them. Without a checkout nothing is known.
//
// Verifies: REQ-SWIFT-012
func TestDependencies(t *testing.T) {
	root, err := filepath.Abs("testdata/repo")
	if err != nil {
		t.Fatal(err)
	}
	r := newResolver(root, langtest.Files(t, root))
	got := r.Dependencies(lang.Target{Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-nio", Version: "2.64.0"})
	want := []lang.Target{
		{Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-atomics", Version: "1.2.0", Requested: "1.1.0..<2.0.0", Pinned: true},
		{Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-collections", Version: "1.1.0", Requested: "1.0.0..<2.0.0", Pinned: true},
		{Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-docc-plugin", Version: "1.0.0..<2.0.0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("swift-nio: got %+v\nwant %+v", got, want)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecosystemSwiftPM, Package: "github.com/apple/swift-log", Version: "1.5.4"}); got != nil {
		t.Errorf("swift-log has no checkout: got %+v", got)
	}
}

// Verifies: REQ-SWIFT-006
func TestPackageName(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/apple/swift-nio.git":      "github.com/apple/swift-nio",
		"https://GitHub.com/Alamofire/Alamofire/":     "github.com/Alamofire/Alamofire",
		"git@github.com:pointfreeco/swift-case-paths": "github.com/pointfreeco/swift-case-paths",
		"ssh://git@gitlab.acme.io:2222/ios/kit.git":   "gitlab.acme.io/ios/kit",
		"https://user@bitbucket.org/team/lib.git":     "bitbucket.org/team/lib",
	} {
		if got := packageName(in); got != want {
			t.Errorf("packageName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := identity("git@github.com:pointfreeco/swift-case-paths.git"); got != "swift-case-paths" {
		t.Errorf("identity = %q", got)
	}
}

// Verifies: REQ-SWIFT-008
func TestRequirement(t *testing.T) {
	type requirement struct {
		requirement      string
		pinned, floating bool
	}
	for in, want := range map[string]requirement{
		`url: "u", from: "1.2.3"`:                 {"1.2.3..<2.0.0", false, false},
		`url: "u", .upToNextMajor(from: "0.9.0")`: {"0.9.0..<1.0.0", false, false},
		`url: "u", .upToNextMinor(from: "1.2.3")`: {"1.2.3..<1.3.0", false, false},
		`url: "u", "1.0.0"..<"1.5.0"`:             {"1.0.0..<1.5.0", false, false},
		`url: "u", "1.0.0"..."1.5.0"`:             {"1.0.0...1.5.0", false, false},
		`url: "u", exact: "1.2.3"`:                {"1.2.3", true, false},
		`url: "u", .exact("1.2.3")`:               {"1.2.3", true, false},
		`url: "u", "1.2.3"`:                       {"1.2.3", true, false},
		`url: "u", revision: "abc"`:               {"abc", true, false},
		`url: "u", branch: "main"`:                {"branch main", false, true},
		`url: "u", .branch("develop")`:            {"branch develop", false, true},
		`name: "N", url: "u", from: "5.0.0"`:      {"5.0.0..<6.0.0", false, false},
		`url: "u", from: "1.0.0", traits: ["A"]`:  {"1.0.0..<2.0.0", false, false},
	} {
		parsed := dependencies(".package(" + in + ")")
		if len(parsed) != 1 {
			t.Fatalf("%s: %d dependencies", in, len(parsed))
		}
		d := parsed[0]
		if got := (requirement{d.requirement, d.pinned, d.floating}); got != want {
			t.Errorf("%s: got %+v, want %+v", in, got, want)
		}
	}
}

// Version 1 of Package.resolved nests its pins under object and names the URL
// repositoryURL; a registry pin's location is its id.
//
// Verifies: REQ-SWIFT-009
func TestResolvedFormats(t *testing.T) {
	v1 := `{"object":{"pins":[{"package":"SnapKit","repositoryURL":"https://github.com/SnapKit/SnapKit.git","state":{"branch":null,"revision":"f222cbd","version":"5.6.0"}}]},"version":1}`
	v2 := `{"pins":[{"identity":"linkedlist","kind":"registry","location":"mona.LinkedList","state":{"version":"1.2.0"}}],"version":2}`
	got := append(readResolved([]byte(v1)), readResolved([]byte(v2))...)
	want := []pin{
		{identity: "snapkit", location: "https://github.com/SnapKit/SnapKit.git", version: "5.6.0", revision: "f222cbd"},
		{identity: "linkedlist", location: "mona.LinkedList", version: "1.2.0", registry: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

// An Xcode project's local package reference is a directory, relative to the
// project's directory.
//
// Verifies: REQ-SWIFT-010
func TestXcodeLocalPackage(t *testing.T) {
	source := `
		C1000000000000000000000D /* XCLocalSwiftPackageReference "Packages/Kit" */ = {
			isa = XCLocalSwiftPackageReference;
			relativePath = Packages/Kit;
		};
		C2000000000000000000000E /* XCRemoteSwiftPackageReference "x" */ = {
			isa = XCRemoteSwiftPackageReference;
			repositoryURL = "https://github.com/a/x";
			requirement = {
				kind = branch;
				branch = main;
			};
		};`
	dependencies, _ := xcodePackages(source)
	want := []dependency{
		{path: "Packages/Kit", line: 2},
		{url: "https://github.com/a/x", requirement: "branch main", floating: true, line: 6},
	}
	if !reflect.DeepEqual(dependencies, want) {
		t.Errorf("got %+v\nwant %+v", dependencies, want)
	}
}

// Verifies: REQ-SWIFT-002
func TestParseImport(t *testing.T) {
	for in, want := range map[string]string{
		"import Foundation":                         "Foundation",
		"@testable import App":                      "App",
		"@_spi(Private) @preconcurrency import Kit": "Kit",
		"public import Core":                        "Core",
		"import func Darwin.sqrt":                   "Darwin",
		"import os.log":                             "os",
		"import struct Collections.Deque;":          "Collections",
		"importFoo":                                 "",
	} {
		rawImport, ok := parseImport(in)
		if rawImport.Module != want || ok != (want != "") {
			t.Errorf("%q: got %q %v, want %q", in, rawImport.Module, ok, want)
		}
	}
}

// The scanner reads imports, declarations and type uses around what would trip a
// tokenizer: nested comments, interpolations (read as code), raw and multi-line
// strings, regex literals, macros, `#if` branches (both read, or only the first
// when they open braces differently) and generic expressions.
//
// Verifies: REQ-SWIFT-002, REQ-SWIFT-003, REQ-SWIFT-011, REQ-SWIFT-014
func TestScanner(t *testing.T) {
	source := `import A
/* /* import Nested */ still a comment */
#if canImport(UIKit)
import UIKit
#elseif os(macOS)
@preconcurrency import AppKit
#endif
let quote = #"a "raw" \(NotCode) \#(Raw.value)"#
let text = """
    import NotAnImport \(Interp.value) "
    """
let re = /"[a-z]+"/
#if os(iOS)
extension Box: UIView {
#else
extension Box: NSView {
#endif
    func draw() { #expect(Macro.x) }
}
struct Box<T: Hashable> where T: Sendable {
    var cache = [Key: Value]()
    var calls = Set<Item>()
    static func < (a: Box, b: Box) -> Bool { a.v < b.v }
    enum Kind { case plain, boxed(Inner, label: Other = .none) }
}
`
	extraction, err := Plugin{}.Extract(&scan.File{Path: "x.swift"}, []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	var imports []string
	references := map[string]int{}
	for _, rawImport := range extraction.Imports {
		if rawImport.Name == kindImport {
			imports = append(imports, rawImport.Spec)
		} else {
			references[rawImport.Spec] = rawImport.Line
		}
	}
	if want := []string{"import A", "import UIKit", "@preconcurrency import AppKit"}; !reflect.DeepEqual(imports, want) {
		t.Errorf("imports %q, want %q", imports, want)
	}
	for name, line := range map[string]int{"Raw": 8, "Interp": 10, "UIView": 14, "Macro": 18, "Hashable": 20, "Sendable": 20, "Item": 22, "Inner": 24, "Other": 24} {
		if references[name] != line {
			t.Errorf("ref %s at %d, want %d", name, references[name], line)
		}
	}
	for _, name := range []string{"NotCode", "NotAnImport", "NSView", "Key", "Value", "Box", "Kind", "T"} {
		if _, ok := references[name]; ok {
			t.Errorf("unexpected ref %s", name)
		}
	}
	symbols := map[string]string{}
	for _, s := range extraction.Symbols {
		symbols[s.Name] = s.Kind
	}
	want := map[string]string{"quote": "var", "text": "var", "re": "var", "Box": "extension", "Box.draw": "method",
		"Box@20": "type", "Box.cache": "property", "Box.calls": "property", "Box.Kind": "type"}
	if !reflect.DeepEqual(symbols, want) {
		t.Errorf("symbols %v\nwant %v", symbols, want)
	}
}

// Whatever a file holds, cut anywhere, the scanner returns: every prefix of a file
// full of unfinished constructs extracts without a panic.
//
// Verifies: REQ-SWIFT-014
func TestTruncated(t *testing.T) {
	source := []byte(`@testable import A.B
extension Array<Box<Int>>: P where Element == Int { }
func f<T: P & Q>(_ x: inout [String: (Int) async throws(E) -> Void] = [:], y: T...) -> some View {
    let s = "\(a + "\(b)") \( { $0 } (1) )" + #"\#(c)"# + """
        \(d)
        """
    let r = #/a\/b/# ; let q = x / y / z
    guard let v = w as? Foo<Bar>, case .a(let z) = v else { return }
    for i: Int in 0..<n where i > 0 { _ = Foo<A, B>() ?? [Baz].init() }
    let c = { [weak self] (a: A, b) -> B in a }
    /* /* unterminated
}
`)
	for i := range len(source) + 1 {
		if _, err := (Plugin{}).Extract(&scan.File{Path: "x.swift"}, source[:i]); err != nil {
			t.Fatalf("prefix %d: %v", i, err)
		}
	}
}

// Verifies: REQ-SWIFT-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"Sources/App/App.swift":                    true,
		"Package.swift":                            true,
		"Package@swift-5.9.swift":                  true,
		".build/checkouts/swift-nio/Package.swift": false,
		"sub/.build/x.swift":                       false,
		"README.md":                                false,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("Claims(%s) = %v", p, got)
		}
	}
}
