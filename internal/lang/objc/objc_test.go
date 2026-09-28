package objc

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cocoapods"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/lang/swift"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo: an iOS app with a Podfile (a method of shared pods, a subspec, an
// exact and a ranged version, git pods by tag, branch and commit, a path pod) and
// its Podfile.lock; a pod's header checked into Pods/; a library podspec (Ruby,
// with subspecs and its own module name) and one in JSON; a Carthage app with a
// Cartfile, its Cartfile.resolved and a built framework; a MATLAB and a Mercury ".m"
// file and a C header, none of them Objective-C.
//
// Verifies: REQ-OBJC-001, REQ-OBJC-002, REQ-OBJC-003, REQ-OBJC-004, REQ-OBJC-005
// Verifies: REQ-OBJC-006, REQ-OBJC-007, REQ-OBJC-008, REQ-OBJC-009, REQ-OBJC-010
// Verifies: REQ-OBJC-011, REQ-OBJC-014
func TestAppAndLibraries(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	imports := map[string]map[string]lang.Target{
		"App/AppDelegate.h": {
			`#import <UIKit/UIKit.h>`: {Ecosystem: swift.AppleEcosystem, Package: "UIKit"},
		},
		"App/AppDelegate.m": {
			`#import "AppDelegate.h"`:                     {Local: "App/AppDelegate.h"},
			`#import "Models/Cart.h"`:                     {Local: "App/Models/Cart.h"},
			`#import "Models/Cart+Pricing.h"`:             {Local: "App/Models/Cart+Pricing.h"},
			`#import <AFNetworking/AFNetworking.h>`:       {Ecosystem: cocoapods.Ecosystem, Package: "AFNetworking", Version: "4.0.1", Requested: "~> 4.0", Pinned: true},
			`#import <SDWebImage/UIImageView+WebCache.h>`: {Ecosystem: cocoapods.Ecosystem, Package: "SDWebImage", Version: "5.18.0", Pinned: true},
			`#import "Masonry.h"`:                         {Ecosystem: cocoapods.Ecosystem, Package: "Masonry", Version: "1.1.0", Pinned: true},
			`#import "AFURLSessionManager.h"`:             {Ecosystem: cocoapods.Ecosystem, Package: "AFNetworking", Version: "4.0.1", Requested: "~> 4.0", Pinned: true},
			`#import <GRDB/GRDB.h>`:                       {Ecosystem: cocoapods.Ecosystem, Package: "GRDB.swift", Version: "6.24.0", Pinned: true},
			`#import <LocalKit/LKThing.h>`:                {Local: "LocalKit/Sources/LKThing.h"},
			`#import <objc/runtime.h>`:                    {Ecosystem: swift.AppleEcosystem, Package: "ObjectiveC"},
			`#import <Appkit/Appkit.h>`:                   {Ecosystem: swift.AppleEcosystem, Package: "AppKit"},
			`#include <stdio.h>`:                          {Ecosystem: "c-std", Package: "stdio.h"},
			`#import <Unknown/Unknown.h>`:                 {Ecosystem: cocoapods.Ecosystem, Package: "Unknown", Unresolved: true},
			`#import "Generated.h"`:                       {},
			`@import Firebase;`:                           {Ecosystem: cocoapods.Ecosystem, Package: "Firebase", Version: "10.0.0", Pinned: true},
			`@import CoreData.NSManagedObject;`:           {Ecosystem: swift.AppleEcosystem, Package: "CoreData"},
		},
		"App/Models/Cart+Pricing.h": {
			`#import "Cart.h"`: {Local: "App/Models/Cart.h"},
		},
		"App/Models/Cart+Pricing.m": {
			`#import "Cart+Pricing.h"`: {Local: "App/Models/Cart+Pricing.h"},
		},
		"App/Models/Cart.h": {
			`#import <Foundation/Foundation.h>`: {Ecosystem: swift.AppleEcosystem, Package: "Foundation"},
		},
		"App/Models/Cart.m": {
			`#import "Cart.h"`: {Local: "App/Models/Cart.h"},
		},
		"App/Models/Store.mm": {
			`#import "Cart.h"`:     {Local: "App/Models/Cart.h"},
			`#include <vector>`:    {Ecosystem: "cpp-std", Package: "vector"},
			`#include "Store.hpp"`: {Local: "App/Models/Store.hpp"},
		},
		"CarthageApp/Cartfile": {
			`github "Mantle/Mantle"`:                             {Ecosystem: cocoapods.Carthage, Package: "github.com/Mantle/Mantle", Version: "2.2.0", Requested: "~> 2.2", Pinned: true},
			`git "https://git.example.com/ios/ReactiveObjC.git"`: {Ecosystem: cocoapods.Carthage, Package: "git.example.com/ios/ReactiveObjC", Version: "3f2d9a1c0b6e4f8a7d5c3b2a1908f7e6d5c4b3a2", Requested: "main", Pinned: true},
			`github "pinterest/PINCache"`:                        {Ecosystem: cocoapods.Carthage, Package: "github.com/pinterest/PINCache", Version: "3.0.3", Pinned: true},
			`binary "https://dl.example.com/Analytics.json"`:     {Ecosystem: cocoapods.Carthage, Package: "dl.example.com/Analytics.json", Version: ">= 1.0", Floating: true},
			`github "acme/Pinned"`:                               {Ecosystem: cocoapods.Carthage, Package: "github.com/acme/Pinned", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true},
		},
		"CarthageApp/Carthage/Build/iOS/Pinned.framework/Headers/Pinned.h": {
			`#import <Foundation/Foundation.h>`: {Ecosystem: swift.AppleEcosystem, Package: "Foundation"},
		},
		"CarthageApp/Sources/Net.m": {
			`#import <Mantle/Mantle.h>`:                                         {Ecosystem: cocoapods.Carthage, Package: "github.com/Mantle/Mantle", Version: "2.2.0", Requested: "~> 2.2", Pinned: true},
			`#import <ReactiveObjC/ReactiveObjC.h>`:                             {Ecosystem: cocoapods.Carthage, Package: "git.example.com/ios/ReactiveObjC", Version: "3f2d9a1c0b6e4f8a7d5c3b2a1908f7e6d5c4b3a2", Requested: "main", Pinned: true},
			`#import <PINCache/PINCache.h>`:                                     {Ecosystem: cocoapods.Carthage, Package: "github.com/pinterest/PINCache", Version: "3.0.3", Pinned: true},
			`@import Analytics;`:                                                {Ecosystem: cocoapods.Carthage, Package: "dl.example.com/Analytics.json", Version: ">= 1.0", Floating: true},
			`#import "../Carthage/Build/iOS/Pinned.framework/Headers/Pinned.h"`: {Ecosystem: cocoapods.Carthage, Package: "github.com/acme/Pinned", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true},
		},
		"Json/Toolkit.podspec.json": {
			`dependency 'SDWebImage'`:      {Ecosystem: cocoapods.Ecosystem, Package: "SDWebImage", Version: "5.18.0", Requested: "~> 5.0", Pinned: true},
			`dependency 'CocoaLumberjack'`: {Ecosystem: cocoapods.Ecosystem, Package: "CocoaLumberjack", Version: "3.8.0", Pinned: true},
		},
		"Lib/ShopKit.podspec": {
			`dependency 'AFNetworking'`:           {Ecosystem: cocoapods.Ecosystem, Package: "AFNetworking", Version: "4.0.1", Requested: "~> 4.0", Pinned: true},
			`dependency 'Mantle'`:                 {Ecosystem: cocoapods.Ecosystem, Package: "Mantle", Version: "2.2.0", Pinned: true},
			`dependency 'ShopKit/Core'`:           {},
			`dependency 'PromiseKit/CorePromise'`: {Ecosystem: cocoapods.Ecosystem, Package: "PromiseKit", Version: ">= 6.0, < 7.0", Floating: true},
		},
		"Lib/Sources/SKOrder.h": {
			`#import <Foundation/Foundation.h>`: {Ecosystem: swift.AppleEcosystem, Package: "Foundation"},
			`#import <Mantle/Mantle.h>`:         {Ecosystem: cocoapods.Ecosystem, Package: "Mantle", Version: "2.2.0", Pinned: true},
			`#import <PromiseKit/PromiseKit.h>`: {Ecosystem: cocoapods.Ecosystem, Package: "PromiseKit", Version: ">= 6.0, < 7.0", Floating: true},
			`@import Shop;`:                     {Local: "Lib/ShopKit.podspec"},
		},
		"LocalKit/LocalKit.podspec": {
			`dependency 'AFNetworking/NSURLSession'`: {Ecosystem: cocoapods.Ecosystem, Package: "AFNetworking", Version: "4.0.1", Requested: "~> 4.0", Pinned: true},
		},
		"LocalKit/Sources/LKThing.h": {
			`#import <Foundation/Foundation.h>`: {Ecosystem: swift.AppleEcosystem, Package: "Foundation"},
		},
		"Podfile": {
			`pod 'AFNetworking'`:       {Ecosystem: cocoapods.Ecosystem, Package: "AFNetworking", Version: "4.0.1", Requested: "~> 4.0", Pinned: true},
			`pod 'SDWebImage'`:         {Ecosystem: cocoapods.Ecosystem, Package: "SDWebImage", Version: "5.18.0", Pinned: true},
			`pod 'Firebase/Analytics'`: {Ecosystem: cocoapods.Ecosystem, Package: "Firebase", Version: "10.0.0", Pinned: true},
			`pod 'Masonry'`:            {Ecosystem: cocoapods.Ecosystem, Package: "Masonry", Version: "1.1.0", Pinned: true},
			`pod 'GRDB.swift'`:         {Ecosystem: cocoapods.Ecosystem, Package: "GRDB.swift", Version: "6.24.0", Pinned: true},
			`pod 'Alamofire'`:          {Ecosystem: cocoapods.Ecosystem, Package: "Alamofire", Version: "5.8.1", Origin: "https://github.com/Alamofire/Alamofire.git"},
			`pod 'AcmeKit'`:            {Ecosystem: cocoapods.Ecosystem, Package: "AcmeKit", Version: "89abcdef0123456789abcdef0123456789abcdef", Pinned: true, Origin: "https://github.com/acme/AcmeKit.git"},
			`pod 'LocalKit'`:           {Local: "LocalKit/LocalKit.podspec"},
			`pod 'Lottie'`:             {Ecosystem: cocoapods.Ecosystem, Package: "Lottie", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true, Origin: "https://github.com/airbnb/lottie-ios.git"},
			`pod 'OCMock'`:             {Ecosystem: cocoapods.Ecosystem, Package: "OCMock", Version: "3.9.1", Requested: ">= 3.0", Pinned: true},
		},
		"Pods/AFNetworking/AFNetworking/AFURLSessionManager.h": {
			`#import <Foundation/Foundation.h>`: {Ecosystem: swift.AppleEcosystem, Package: "Foundation"},
		},
	}
	symbols := map[string]map[string]string{
		"App/AppDelegate.h":         {"AppDelegate": "class", "AppDelegate.window": "property", "AppDelegate.onLaunch": "property", "AppDelegate.shared": "method", "AppDelegate.openCart:animated:": "method"},
		"App/AppDelegate.m":         {"kLaunchKey": "const", "AppDelegate.launches": "property", "AppDelegate": "class", "AppDelegate.shared": "method", "AppDelegate.openCart:animated:": "method", "AppDelegate.track": "method"},
		"App/Models/Cart+Pricing.h": {"Cart(Pricing)": "extension", "Cart.total": "method"},
		"App/Models/Cart+Pricing.m": {"Cart(Pricing)": "extension", "Cart.total": "method"},
		"App/Models/Cart.h":         {"CART_MAX_ITEMS": "macro", "CART_LOG": "macro", "CartState": "enum", "CartFlags": "enum", "CartCompletion": "type", "CartTotal": "struct", "CartDidChangeNotification": "const", "CartTotalMake": "func", "CartObserver": "interface", "CartObserver.cartDidChange:": "method", "CartObserver.cart:didAdd:": "method", "Cart": "class", "Cart.items": "property", "Cart.current": "property", "Cart.initWithItems:": "method", "Cart.addItem::": "method", "Cart.formatted:": "method"},
		"App/Models/Cart.m":         {"CartDidChangeNotification": "const", "CartTotalMake": "func", "CartIsEmpty": "func", "Cart": "class", "Cart.initWithItems:": "method", "Cart.addItem::": "method", "Cart.formatted:": "method", "Cart.items": "method"},
		"App/Models/Store.mm":       {"Ledger": "class", "sum": "func", "Store": "class", "Store.checkout": "method"},
		"CarthageApp/Cartfile":      {},
		"CarthageApp/Carthage/Build/iOS/Pinned.framework/Headers/Pinned.h": {"PinnedThing": "class"},
		"CarthageApp/Sources/Net.m":                                        {"CAStart": "func"},
		"Json/Toolkit.podspec.json":                                        {},
		"Lib/ShopKit.podspec":                                              {},
		"Lib/Sources/SKOrder.h":                                            {"SKOrder": "class"},
		"LocalKit/LocalKit.podspec":                                        {},
		"LocalKit/Sources/LKThing.h":                                       {"LKThing": "class"},
		"Podfile":                                                          {},
		"Pods/AFNetworking/AFNetworking/AFURLSessionManager.h":             {"AFURLSessionManager": "class"},
	}
	var got []string
	for f := range res {
		got = append(got, f)
	}
	sort.Strings(got)
	var want []string
	for f := range imports {
		want = append(want, f)
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("analyzed %v\nwant %v", got, want)
	}
	for f, w := range imports {
		t.Run(f, func(t *testing.T) {
			langtest.CheckImports(t, res[f], w)
			langtest.CheckSymbols(t, res[f], symbols[f])
		})
	}
}

// ".m" is also MATLAB's and Mercury's extension and ".h" C's: the scan tells them
// apart by content, so the plugin takes only Objective-C, and the cpp plugin leaves
// an Objective-C header alone.
//
// Verifies: REQ-OBJC-001
func TestClaimsOnlyObjectiveC(t *testing.T) {
	files := langtest.Files(t, "testdata/repo")
	claimed := func(p lang.Plugin) map[string]bool {
		out := map[string]bool{}
		for _, f := range lang.Claimed(p, files) {
			out[f.Path] = true
		}
		return out
	}
	objc, c := claimed(Plugin{}), claimed(cpp.Plugin{})
	for path, want := range map[string]bool{
		"App/AppDelegate.h": true, "App/AppDelegate.m": true, "App/Models/Store.mm": true,
		"Podfile": true, "Lib/ShopKit.podspec": true, "Json/Toolkit.podspec.json": true,
		"CarthageApp/Cartfile": true, "Podfile.lock": false, "CarthageApp/Cartfile.resolved": false,
		"Vendor/Legacy/smooth.m": false, "Vendor/Legacy/solver.m": false, "Vendor/Legacy/util.h": false,
		"App/Models/Store.hpp": false, "App/Swift/Bridge.swift": false,
	} {
		if objc[path] != want {
			t.Errorf("objc claims %s: %v, want %v", path, objc[path], want)
		}
	}
	for path, want := range map[string]bool{"Vendor/Legacy/util.h": true, "App/Models/Store.hpp": true, "App/AppDelegate.h": false, "App/Models/Cart.h": false} {
		if c[path] != want {
			t.Errorf("cpp claims %s: %v, want %v", path, c[path], want)
		}
	}
	// A manifest's extraction depends on its name, which the cache key carries.
	var podfile, cartfile *scan.File
	for _, f := range files {
		switch f.Path {
		case "Podfile":
			podfile = f
		case "CarthageApp/Cartfile":
			cartfile = f
		}
	}
	if lang.ClassOf(Plugin{}, podfile) == lang.ClassOf(Plugin{}, cartfile) {
		t.Error("a Podfile and a Cartfile share a cache class")
	}
}

// Podfile.lock says what each pod's specs depend on: that is the transitive walk,
// pinned by the same lock, subspecs folded into their pod.
//
// Verifies: REQ-OBJC-008
func TestLockGraph(t *testing.T) {
	r, _ := Plugin{}.Resolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	tr := r.(lang.Transitive)
	for pod, want := range map[string][]lang.Target{
		"Firebase": {
			{Ecosystem: cocoapods.Ecosystem, Package: "FirebaseAnalytics", Version: "10.0.0", Pinned: true},
			{Ecosystem: cocoapods.Ecosystem, Package: "FirebaseCore", Version: "10.0.0", Pinned: true},
		},
		"FirebaseAnalytics": {
			{Ecosystem: cocoapods.Ecosystem, Package: "FirebaseCore", Version: "10.0.0", Pinned: true},
			{Ecosystem: cocoapods.Ecosystem, Package: "GoogleUtilities", Version: "7.11.0", Pinned: true},
		},
		"LocalKit": {
			{Ecosystem: cocoapods.Ecosystem, Package: "AFNetworking", Version: "4.0.1", Requested: "~> 4.0", Pinned: true},
		},
		"AFNetworking": nil,
	} {
		got := tr.Dependencies(lang.Target{Ecosystem: cocoapods.Ecosystem, Package: pod})
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s depends on %+v\nwant %+v", pod, got, want)
		}
	}
}

// The Objective-C and the Swift resolvers both answer for a Carthage dependency
// from its checkout, and say the answer comes from what is installed.
//
// Verifies: REQ-OBJC-011
func TestCarthageCheckoutDependencies(t *testing.T) {
	root := langtest.Write(t, map[string]string{
		"Cartfile":          `github "Alamofire/AlamofireImage" ~> 4.0` + "\n",
		"Cartfile.resolved": `github "Alamofire/Alamofire" "5.8.1"` + "\n" + `github "Alamofire/AlamofireImage" "4.3.0"` + "\n",
		"Carthage/Checkouts/AlamofireImage/Cartfile": `github "Alamofire/Alamofire" ~> 5.0` + "\n",
	})
	image := lang.Target{Ecosystem: cocoapods.Carthage, Package: "github.com/Alamofire/AlamofireImage"}
	want := []lang.Target{{Ecosystem: cocoapods.Carthage, Package: "github.com/Alamofire/Alamofire", Version: "5.8.1", Pinned: true}}
	for _, p := range []lang.Plugin{Plugin{}, swift.Plugin{}} {
		r, _ := p.Resolver(root, langtest.Files(t, root))
		if got := r.(lang.Transitive).Dependencies(image); !reflect.DeepEqual(got, want) || !r.(lang.Installed).Installed(image) {
			t.Errorf("%s: AlamofireImage depends on %+v", p.Name(), got)
		}
	}
}

// A Swift file of an app built with CocoaPods imports its pods by module name.
//
// Verifies: REQ-OBJC-012, REQ-OBJC-014
func TestSwiftImportsPods(t *testing.T) {
	res := langtest.Analyze(t, swift.Plugin{}, "testdata/repo")
	langtest.CheckImports(t, res["App/Swift/Bridge.swift"], map[string]lang.Target{
		`import UIKit`:        {Ecosystem: swift.AppleEcosystem, Package: "UIKit"},
		`import Alamofire`:    {Ecosystem: cocoapods.Ecosystem, Package: "Alamofire", Version: "5.8.1", Origin: "https://github.com/Alamofire/Alamofire.git"},
		`import GRDB`:         {Ecosystem: cocoapods.Ecosystem, Package: "GRDB.swift", Version: "6.24.0", Pinned: true},
		`import Lottie`:       {Ecosystem: cocoapods.Ecosystem, Package: "Lottie", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true, Origin: "https://github.com/airbnb/lottie-ios.git"},
		`import FirebaseCore`: {Ecosystem: cocoapods.Ecosystem, Package: "FirebaseCore", Version: "10.0.0", Pinned: true},
		`import Kingfisher`:   {Ecosystem: "swiftpm", Package: "github.com/onevcat/Kingfisher", Unresolved: true},
	})
}

// Objective-C source is read by a scanner that must never fail on what it is given:
// a panic in Extract would end the whole analysis. Cut files and odd syntax come out
// as whatever can be read.
//
// Verifies: REQ-OBJC-013
func TestScannerSurvivesBrokenSource(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "repo", "App", "Models", "Cart.h"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= len(src); i++ {
		if _, err := (Plugin{}).Extract(&scan.File{Path: "Cart.h"}, src[:i]); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []string{"@interface", "- (", "@implementation X\n- (void)a {", "typedef NS_ENUM(", "R\"x(", "@\"", "'", "#define", "@property (", "}}}}", "(((("} {
		if _, err := (Plugin{}).Extract(&scan.File{Path: "x.m"}, []byte(s)); err != nil {
			t.Fatal(err)
		}
	}
}

// An Xcode project's header search path tells apart headers that two directories
// hold: without it <Lib/Lib.h> is taken to the shallowest directory named Lib, and
// "Config.h" to nothing.
//
// Verifies: REQ-OBJC-004, REQ-OBJC-015
func TestXcodeHeaderSearchPaths(t *testing.T) {
	files := map[string]string{
		"App/main.m":            "#import <Lib/Lib.h>\n#import \"Config.h\"\n",
		"Vendor/x/Lib/Lib.h":    "",
		"Settings/Config.h":     "",
		"Other/Lib/Lib.h":       "",
		"Other/Config.h":        "",
		"Settings/App.xcconfig": "USER_HEADER_SEARCH_PATHS = $(inherited) \"$(PROJECT_DIR)/Settings\"\n",
		"App.xcodeproj/project.pbxproj": "{ objects = { 1 = { isa = XCBuildConfiguration; buildSettings = {\n" +
			"HEADER_SEARCH_PATHS = (\"$(inherited)\", \"$(SRCROOT)/Vendor/**\"); }; }; }; }\n",
	}
	res := langtest.Analyze(t, Plugin{}, langtest.Write(t, files))
	langtest.CheckImports(t, res["App/main.m"], map[string]lang.Target{
		"#import <Lib/Lib.h>":  {Local: "Vendor/x/Lib/Lib.h"},
		"#import \"Config.h\"": {Local: "Settings/Config.h"},
	})
	delete(files, "App.xcodeproj/project.pbxproj")
	delete(files, "Settings/App.xcconfig")
	res = langtest.Analyze(t, Plugin{}, langtest.Write(t, files))
	langtest.CheckImports(t, res["App/main.m"], map[string]lang.Target{
		"#import <Lib/Lib.h>":  {Local: "Other/Lib"},
		"#import \"Config.h\"": {},
	})
}
