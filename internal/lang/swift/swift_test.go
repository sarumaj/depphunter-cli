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
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	imports := map[string]map[string]lang.Target{
		"App/App/AppModel.swift": {
			`import Combine`:   {Ecosystem: ecoApple, Package: "Combine"},
			`MainActor`:        {},
			`ObservableObject`: {},
			`Published`:        {},
		},
		"App/App/ContentView.swift": {
			`import SwiftUI`:    {Ecosystem: ecoApple, Package: "SwiftUI"},
			`import Alamofire`:  {Ecosystem: ecoSwiftPM, Package: "github.com/Alamofire/Alamofire", Version: "5.8.1", Requested: "5.8.0..<6.0.0", Pinned: true},
			`import Kingfisher`: {Ecosystem: ecoSwiftPM, Package: "github.com/onevcat/Kingfisher", Version: "7.10.0", Pinned: true},
			`import CoreKit`:    {Local: "App/CoreKit"},
			`View`:              {},
			`AppModel`:          {Local: "App/App/AppModel.swift"},
			`StateObject`:       {},
			`CoreThing`:         {Local: "App/CoreKit/Core.swift"},
			`Text`:              {},
		},
		"App/AppTests/AppTests.swift": {
			`import XCTest`:        {Ecosystem: ecoStd, Package: "XCTest"},
			`@testable import App`: {Local: "App"},
			`import CoreKit`:       {Local: "App/CoreKit"},
			`XCTestCase`:           {},
			`AppModel`:             {Local: "App/App/AppModel.swift"},
			`CoreThing`:            {Local: "App/CoreKit/Core.swift"},
		},
		"App/CoreKit/Core.swift": {},
		"Local/LocalKit/Package.swift": {
			`import PackageDescription`: {Ecosystem: ecoStd, Package: "PackageDescription"},
			`Package`:                   {},
		},
		"Local/LocalKit/Sources/LocalKit/LocalKit.swift": {},
		"Package.swift": {
			`import PackageDescription`: {Ecosystem: ecoStd, Package: "PackageDescription"},
			`Package`:                   {},
			`.package(url: "https://github.com/apple/swift-nio.git", from: "2.60.0")`:                                    {Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-nio", Version: "2.64.0", Requested: "2.60.0..<3.0.0", Pinned: true},
			`.package(url: "https://github.com/apple/swift-log", .upToNextMajor(from: "1.5.0"))`:                         {Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-log", Version: "1.5.4", Requested: "1.5.0..<2.0.0", Pinned: true},
			`.package(url: "git@github.com:apple/swift-argument-parser.git", exact: "1.3.0")`:                            {Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-argument-parser", Version: "1.3.0", Pinned: true},
			`.package(url: "https://github.com/apple/swift-collections", "1.0.0"..<"2.0.0")`:                             {Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-collections", Version: "1.1.0", Requested: "1.0.0..<2.0.0", Pinned: true},
			`.package(url: "https://github.com/swiftlang/swift-markdown.git", branch: "main")`:                           {Ecosystem: ecoSwiftPM, Package: "github.com/swiftlang/swift-markdown", Version: "4aae40bf6fff5286e0e1672329d17824ce16e081", Requested: "branch main", Pinned: true},
			`.package(url: "https://github.com/acme/private-kit", revision: "0123456789abcdef0123456789abcdef01234567")`: {Ecosystem: ecoSwiftPM, Package: "github.com/acme/private-kit", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true},
			`.package(path: "Local/LocalKit")`:                                                                           {Local: "Local/LocalKit"},
			`.package(id: "mona.LinkedList", from: "1.0.0")`:                                                             {Ecosystem: ecoSwiftPM, Package: "mona.LinkedList", Version: "1.0.0..<2.0.0"},
		},
		"Sources/Demo/Model.swift": {
			`String`: {},
			`Void`:   {},
		},
		"Sources/Demo/Server.swift": {
			`import Foundation`:               {Ecosystem: ecoStd, Package: "Foundation"},
			`import NIOCore`:                  {Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-nio", Version: "2.64.0", Requested: "2.60.0..<3.0.0", Pinned: true},
			`@preconcurrency import Logging`:  {Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-log", Version: "1.5.4", Requested: "1.5.0..<2.0.0", Pinned: true},
			`@_exported import Util`:          {Local: "Sources/Util"},
			`import struct Collections.Deque`: {Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-collections", Version: "1.1.0", Requested: "1.0.0..<2.0.0", Pinned: true},
			`import Markdown`:                 {Ecosystem: ecoSwiftPM, Package: "github.com/swiftlang/swift-markdown", Version: "4aae40bf6fff5286e0e1672329d17824ce16e081", Requested: "branch main", Pinned: true},
			`import UIKit`:                    {Ecosystem: ecoApple, Package: "UIKit"},
			`import AppKit`:                   {Ecosystem: ecoApple, Package: "AppKit"},
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
			`import Collections`: {Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-collections", Version: "1.1.0", Requested: "1.0.0..<2.0.0", Pinned: true},
			`import LinkedList`:  {Ecosystem: ecoSwiftPM, Package: "mona.LinkedList", Version: "1.0.0..<2.0.0"},
			`Array`:              {},
			`Int`:                {},
		},
		"Sources/demo-cli/main.swift": {
			`import ArgumentParser`: {Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-argument-parser", Version: "1.3.0", Pinned: true},
			`import Demo`:           {Local: "Sources/Demo"},
			`import SnapKit`:        {Ecosystem: ecoSwiftPM, Package: "github.com/SnapKit/SnapKit", Unresolved: true},
			`import Unknown`:        {Ecosystem: ecoSwiftPM, Package: "Unknown", Unresolved: true},
			`Server`:                {Local: "Sources/Demo/Server.swift"},
		},
		"Tests/DemoTests/ServerTests.swift": {
			`import XCTest`:         {Ecosystem: ecoStd, Package: "XCTest"},
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
	for file, r := range res {
		t.Run(file, func(t *testing.T) {
			langtest.CheckImports(t, r, imports[file])
			langtest.CheckSymbols(t, r, symbols[file])
		})
	}
	for file := range symbols {
		if res[file] == nil {
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
	got := r.Dependencies(lang.Target{Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-nio", Version: "2.64.0"})
	want := []lang.Target{
		{Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-atomics", Version: "1.2.0", Requested: "1.1.0..<2.0.0", Pinned: true},
		{Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-collections", Version: "1.1.0", Requested: "1.0.0..<2.0.0", Pinned: true},
		{Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-docc-plugin", Version: "1.0.0..<2.0.0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("swift-nio: got %+v\nwant %+v", got, want)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecoSwiftPM, Package: "github.com/apple/swift-log", Version: "1.5.4"}); got != nil {
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
	type req struct {
		requirement      string
		pinned, floating bool
	}
	for in, want := range map[string]req{
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
		deps := dependencies(".package(" + in + ")")
		if len(deps) != 1 {
			t.Fatalf("%s: %d dependencies", in, len(deps))
		}
		d := deps[0]
		if got := (req{d.requirement, d.pinned, d.floating}); got != want {
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
	src := `
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
	deps, _ := xcodePackages(src)
	want := []dependency{
		{path: "Packages/Kit", line: 2},
		{url: "https://github.com/a/x", requirement: "branch main", floating: true, line: 6},
	}
	if !reflect.DeepEqual(deps, want) {
		t.Errorf("got %+v\nwant %+v", deps, want)
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
		imp, ok := parseImport(in)
		if imp.Module != want || ok != (want != "") {
			t.Errorf("%q: got %q %v, want %q", in, imp.Module, ok, want)
		}
	}
	src := "import A\n#if canImport(UIKit)\nimport UIKit\n  #elseif os(macOS)\nimport AppKit\n#else\n#endif\n    #expect(a)\n#Preview {}\nif #available(iOS 15, *) {}\nf { unsafe Array($0) }; let unsafe = 1; nonisolated(unsafe) var x\n"
	want := "import A\n                    \nimport UIKit\n                   \nimport AppKit\n     \n      \n    _expect(a)\n_Preview {}\nif #available(iOS 15, *) {}\nf {        Array($0) }; let unsafe = 1; nonisolated(unsafe) var x\n"
	if got := string(prepare([]byte(src))); got != want {
		t.Errorf("prepare: got %q", got)
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
