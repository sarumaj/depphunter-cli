package cocoapods

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// The pinning rule without a lock: one exact version pins and is shown bare, a range
// floats, and a git pod pins by commit, shows a tag, floats on a branch.
//
// Verifies: REQ-OBJC-009
func TestPinningRule(t *testing.T) {
	for requirements, want := range map[string]lang.Target{
		"1.2.3":          {Version: "1.2.3", Pinned: true},
		"= 1.2.3":        {Version: "1.2.3", Pinned: true},
		"~> 1.2":         {Version: "~> 1.2", Floating: true},
		">= 1.0, < 2.0":  {Version: ">= 1.0, < 2.0", Floating: true},
		"":               {Floating: true},
		"1.2.3-beta.1":   {Version: "1.2.3-beta.1", Pinned: true},
		"= 1.2, = 1.2.3": {Version: "= 1.2, = 1.2.3", Floating: true},
	} {
		var got lang.Target
		requirement(&got, requirements)
		if got != want {
			t.Errorf("%q: %+v, want %+v", requirements, got, want)
		}
	}
	for _, c := range []struct {
		tag, branch, commit string
		want                lang.Target
	}{
		{"", "", "0123456789abcdef0123456789abcdef01234567", lang.Target{Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true}},
		{"v1.0", "", "", lang.Target{Version: "v1.0"}},
		{"", "main", "", lang.Target{Version: "main", Floating: true}},
		{"", "", "", lang.Target{Floating: true}},
	} {
		var got lang.Target
		gitPin(&got, c.tag, c.branch, c.commit)
		if got != c.want {
			t.Errorf("%+v: %+v, want %+v", c, got, c.want)
		}
	}
}

// A module or header directory finds its pod by name when the spellings differ.
//
// Verifies: REQ-OBJC-006
func TestPodMatches(t *testing.T) {
	for _, c := range []struct {
		pod, module string
		want        bool
	}{
		{"libPhoneNumber-iOS", "libPhoneNumber_iOS", true},
		{"lottie-ios", "Lottie", true},
		{"GRDB.swift", "GRDB", true},
		{"ReactiveObjC", "ReactiveObjC", true},
		{"libwebp", "webp", true},
		{"SDWebImage", "SDWebImageWebPCoder", false},
		{"Kit", "KitSwift", false},
	} {
		if got := podMatches(c.pod, fold(c.module)); got != c.want {
			t.Errorf("%s by %s: %v, want %v", c.pod, c.module, got, c.want)
		}
	}
}

// Manifests read as text: Ruby statements across lines, options in both hash
// syntaxes, comments, and Cartfile sources named as the swiftpm island names URLs.
//
// Verifies: REQ-OBJC-007, REQ-OBJC-010, REQ-OBJC-011
func TestManifests(t *testing.T) {
	podfile := readPodfile(`source "https://cdn.cocoapods.org/" # the CDN
pod 'A', '~> 1.0', '< 1.5'
pod "B/Sub", :git => "https://x/b.git", :tag => 'v2' # comment 'quoted'
pod('C',
    path: '../c')
pod "D#{suffix}"
podspec
`)
	if !reflect.DeepEqual(podfile.sources, []string{"https://cdn.cocoapods.org/"}) {
		t.Errorf("sources %v", podfile.sources)
	}
	want := []*declaration{
		{name: "A", requirements: "~> 1.0, < 1.5"},
		{name: "B/Sub", git: "https://x/b.git", tag: "v2"},
		{name: "C", path: "../c"},
	}
	if !reflect.DeepEqual(podfile.pods, want) {
		t.Errorf("pods %+v", podfile.pods)
	}
	if lines := []int{podfile.dependencies[0].Line, podfile.dependencies[1].Line, podfile.dependencies[2].Line}; !reflect.DeepEqual(lines, []int{2, 3, 4}) {
		t.Errorf("lines %v", lines)
	}
	for kind, source := range map[string]string{
		"github:owner/repo":                         "github.com/owner/repo",
		"github:https://ghe.example.com/owner/repo": "ghe.example.com/owner/repo",
		"git:git@github.com:owner/repo.git":         "github.com/owner/repo",
		"binary:https://dl.example.com/Kit.json":    "dl.example.com/Kit.json",
	} {
		k, s, _ := cut(kind)
		if got := cartName(k, s); got != source {
			t.Errorf("%s: %s, want %s", kind, got, source)
		}
	}
}

func cut(s string) (string, string, bool) {
	for i := range s {
		if s[i] == ':' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// A header found in a dependency checkout belongs to the dependency.
//
// Verifies: REQ-OBJC-006
func TestVendored(t *testing.T) {
	x := &Index{own: map[string]string{}}
	for local, want := range map[string]string{
		"Pods/AFNetworking/AFNetworking/AFHTTPSessionManager.h":  "AFNetworking",
		"ios/Pods/Headers/Public/SDWebImage/SDWebImage.h":        "SDWebImage",
		"Carthage/Checkouts/Mantle/Sources/Mantle.h":             "Mantle",
		"Carthage/Build/iOS/Kit.framework/Headers/Kit.h":         "Kit",
		"Pods/Target Support Files/Pods-App/Pods-App-umbrella.h": "",
		"Pods/Headers/x.h":         "",
		"src/Pods.h":               "",
		"App/PodsHelper/Thing.h":   "",
		"Carthage/Build/iOS/x.txt": "",
	} {
		got, ok := x.Vendored("App/a.m", local)
		if (want != "") != ok || got.Package != want {
			t.Errorf("%s: %+v %v, want %q", local, got, ok, want)
		}
	}
}

// AlamofireImage, checked out under Carthage/Checkouts, depends on what its own
// Cartfile names (not its Cartfile.private): Alamofire, pinned by the project's
// Cartfile.resolved, and Kingfisher, which only the checkout's Cartfile.resolved
// pins. A checkout that is missing or not a Cartfile says nothing.
//
// Verifies: REQ-OBJC-011
func TestCarthageCheckouts(t *testing.T) {
	files := map[string]string{
		"App/Cartfile":          `github "Alamofire/AlamofireImage" ~> 4.0` + "\n" + `github "ReactiveX/RxSwift" ~> 6.0` + "\n",
		"App/Cartfile.resolved": `github "Alamofire/Alamofire" "5.8.1"` + "\n" + `github "Alamofire/AlamofireImage" "4.3.0"` + "\n",
		"App/Carthage/Checkouts/AlamofireImage/Cartfile": `github "Alamofire/Alamofire" ~> 5.0` + "\n" +
			`github "onevcat/Kingfisher" ~> 7.0` + "\n",
		"App/Carthage/Checkouts/AlamofireImage/Cartfile.private": `github "Quick/Nimble"` + "\n",
		"App/Carthage/Checkouts/AlamofireImage/Cartfile.resolved": `github "Alamofire/Alamofire" "5.6.0"` + "\n" +
			`github "onevcat/Kingfisher" "7.10.0"` + "\n" + `github "Quick/Nimble" "13.0.0"` + "\n",
		"App/Carthage/Checkouts/RxSwift/Cartfile": "\x00{{ not a Cartfile",
	}
	root := langtest.Write(t, files)
	x := Read(root, langtest.Files(t, root))
	image := lang.Target{Ecosystem: Carthage, Package: "github.com/Alamofire/AlamofireImage", Version: "4.3.0", Pinned: true}
	want := []lang.Target{
		{Ecosystem: Carthage, Package: "github.com/Alamofire/Alamofire", Version: "5.8.1", Pinned: true},
		{Ecosystem: Carthage, Package: "github.com/onevcat/Kingfisher", Version: "7.10.0", Requested: "~> 7.0", Pinned: true},
	}
	if got := x.Dependencies(image); !reflect.DeepEqual(got, want) || !x.Installed(image) {
		t.Errorf("AlamofireImage depends on %+v", got)
	}
	for _, name := range []string{"github.com/ReactiveX/RxSwift", "github.com/Alamofire/Alamofire"} {
		if got := x.Dependencies(lang.Target{Ecosystem: Carthage, Package: name}); len(got) != 0 {
			t.Errorf("%s depends on %+v", name, got)
		}
	}
	if got := Read("", nil).Dependencies(image); got != nil {
		t.Errorf("no root: %+v", got)
	}
}
