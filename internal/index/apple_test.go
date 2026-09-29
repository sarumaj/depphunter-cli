package index

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// podsHome is a home whose CocoaPods repos directory holds an unsharded clone of
// a company spec repository (Specs/ below it, a JSON and a Ruby podspec), a
// sharded clone of the trunk repository without Specs/, and a clone with no
// origin remote, which is no spec repository anybody names.
func podsHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	repos := filepath.Join(home, ".cocoapods", "repos")
	put(t, filepath.Join(repos, "acme", ".git", "config"), "[core]\n\tbare = false\n[remote \"upstream\"]\n\turl = https://elsewhere.test/Specs.git\n"+
		"[remote \"origin\"]\n\turl = https://ci:secret@git.corp.test/iOS/Specs/\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n")
	put(t, filepath.Join(repos, "acme", "Specs", "AcmeKit", "1.2.0", "AcmeKit.podspec.json"),
		`{"name":"AcmeKit","dependencies":{"AFNetworking":["~> 4.0"]},"subspecs":[{"name":"Core","dependencies":{"AcmeKit/Base":[]}}]}`)
	put(t, filepath.Join(repos, "acme", "Specs", "AcmeKit", "1.0.0", "AcmeKit.podspec"), `Pod::Spec.new do |s|
  s.name = 'AcmeKit'
  s.dependency 'Mantle', '= 2.2.0'
  s.test_spec 'Tests' do |test|
    test.dependency 'OCMock'
  end
end
`)
	put(t, filepath.Join(repos, "acme", "Specs", "AcmeKit", "2.0.0-beta.1", "AcmeKit.podspec.json"), `{"name":"AcmeKit","dependencies":{"Beta":[]}}`)
	sum := md5.Sum([]byte("AFNetworking"))
	digits := hex.EncodeToString(sum[:])
	put(t, filepath.Join(repos, "master", ".git", "config"), "[remote \"origin\"]\n\turl = https://github.com/CocoaPods/Specs.git\n")
	put(t, filepath.Join(repos, "master", "CocoaPods-version.yml"), "---\nmin: 1.0.0\nprefix_lengths:\n- 1\n- 1\n- 1\n")
	put(t, filepath.Join(repos, "master", digits[0:1], digits[1:2], digits[2:3], "AFNetworking", "4.0.1", "AFNetworking.podspec.json"),
		`{"name":"AFNetworking","dependencies":{},"subspecs":[{"name":"NSURLSession","dependencies":{"AFNetworking/Security":[]}}]}`)
	put(t, filepath.Join(repos, "loose", ".git", "config"), "[core]\n\tbare = false\n")
	return home
}

// A Podfile's sources are asked in its order, and only they: a company spec
// repository cloned on this machine (`pod repo add`) is read from the clone,
// found by its origin remote the way CocoaPods compares URLs (case, ".git", a
// trailing slash and a credential aside), sharded or not, a Ruby podspec as
// well as a JSON one; the trunk is read from a clone of it before the CDN.
// Nothing goes over the network, and a pod the clone lacks is asked of the next
// source.
//
// Verifies: REQ-SUP-074, REQ-SUP-051
func TestCocoaPodsSpecRepositoryClones(t *testing.T) {
	cdn := newFeed(t, nil)
	asPublic(t, CocoaPods, cdn)
	home := podsHome(t)
	files := write(t, map[string]string{"ios/Podfile": "source 'https://git.corp.test/ios/specs/'\nsource 'https://cdn.cocoapods.org/'\npod 'AcmeKit'\n"})
	config := NewDiscoverer(environment(nil), home).Discover(files)
	if got, want := order(config, CocoaPods, "AcmeKit", ""), []string{"https://git.corp.test/ios/specs", cdn.URL}; !reflect.DeepEqual(got, want) {
		t.Errorf("candidates %v, want %v", got, want)
	}
	c := newClient(t, config)
	for _, testCase := range []struct {
		target lang.Target
		want   []string
		index  string
	}{
		{lang.Target{Package: "AcmeKit", Version: "~> 1.0"}, []string{"AFNetworking"}, "https://git.corp.test/ios/specs"},
		{lang.Target{Package: "AcmeKit", Version: "1.0.0", Pinned: true}, []string{"Mantle"}, "https://git.corp.test/ios/specs"},
		{lang.Target{Package: "AcmeKit"}, []string{"AFNetworking"}, "https://git.corp.test/ios/specs"},
		{lang.Target{Package: "AFNetworking"}, nil, cdn.URL},
	} {
		testCase.target.Ecosystem = CocoaPods
		got, lookup := ask(t, c, testCase.target)
		if !reflect.DeepEqual(got, testCase.want) || lookup.Index != testCase.index || lookup.Answer != trace.FromIndex || len(lookup.Requests) != 0 {
			t.Errorf("%s %s: got %v from %s (%+v), want %v from %s", testCase.target.Package, testCase.target.Version, got, lookup.Index, lookup, testCase.want, testCase.index)
		}
	}
	if got := c.Dependencies(lang.Target{Ecosystem: CocoaPods, Package: "AcmeKit", Version: "1.0.0", Pinned: true}); !reflect.DeepEqual(got,
		[]lang.Target{{Ecosystem: CocoaPods, Package: "Mantle", Version: "2.2.0", Pinned: true}}) {
		t.Errorf("Ruby podspec: %+v", got)
	}
	if _, lookup := ask(t, c, lang.Target{Ecosystem: CocoaPods, Package: "Nowhere", Version: "1.0"}); lookup.Answer != trace.NoAnswer {
		t.Errorf("a pod no clone has: %+v", lookup)
	}
	if paths := cdn.paths(); len(paths) != 0 {
		t.Errorf("the CDN was asked %v", paths)
	}
	for _, name := range []string{"../acme", "AcmeKit/../../x"} {
		if _, err := cocoapodsCopy(filepath.Join(home, ".cocoapods", "repos", "acme"), lang.Target{Package: name}); !notFound(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A clone is a copy of the spec repository a Podfile names, not an index of its
// own: without a Podfile source, CocoaPods asks the trunk alone. A Podfile listing
// only a company repository switches the trunk off; one naming the repository in
// Podfile.lock's SPEC REPOS is known by its clone. A git spec repository with no
// clone, even one the user vouches for, is not read (a note says so) and passes
// the question on.
//
// Verifies: REQ-SUP-074, REQ-SUP-051, REQ-TRC-017
func TestCocoaPodsSourcesAndClones(t *testing.T) {
	cdn := newFeed(t, nil)
	asPublic(t, CocoaPods, cdn)
	home := podsHome(t)
	config := NewDiscoverer(environment(nil), home).Discover(write(t, map[string]string{"Podfile": "pod 'AcmeKit'\n"}))
	if got, want := order(config, CocoaPods, "AcmeKit", ""), []string{cdn.URL}; !reflect.DeepEqual(got, want) {
		t.Errorf("no sources: %v, want %v", got, want)
	}
	config = NewDiscoverer(environment(nil), home).Discover(write(t, map[string]string{"Podfile": "source 'https://git.corp.test/iOS/Specs.git'\n"}))
	if got, want := order(config, CocoaPods, "AcmeKit", ""), []string{"https://git.corp.test/iOS/Specs.git"}; !reflect.DeepEqual(got, want) {
		t.Errorf("a company repository alone: %v, want %v", got, want)
	}
	config = NewDiscoverer(environment(nil), home).Discover(write(t, map[string]string{
		"Podfile.lock": "SPEC REPOS:\n  https://git.corp.test/iOS/Specs.git:\n    - AcmeKit\n  https://git.other.test/Specs.git:\n    - OtherKit\n  trunk:\n    - AFNetworking\n",
	}))
	for packageName, want := range map[string]string{"AcmeKit": "https://git.corp.test/iOS/Specs.git", "OtherKit": "https://git.other.test/Specs.git?", "AFNetworking": cdn.URL} {
		if got := order(config, CocoaPods, packageName, ""); !reflect.DeepEqual(got, []string{want}) {
			t.Errorf("%s: %v, want %s", packageName, got, want)
		}
	}
	for _, s := range config.Report() {
		if s.Ecosystem == CocoaPods && strings.Contains(s.URL, "corp") && !s.Trusted {
			t.Errorf("the cloned repository is reported untrusted: %+v", s)
		}
	}
	config = NewDiscoverer(environment(nil), t.TempDir()).Discover(write(t, map[string]string{"Podfile": "source 'git@git.other.test:ios/Specs.git'\nsource 'https://cdn.cocoapods.org/'\n"}))
	config.Trust([]string{"git@git.other.test:ios/Specs.git"})
	c := newClient(t, config)
	if notes := notesOf(t, c, lang.Target{Ecosystem: CocoaPods, Package: "OtherKit", Version: "1.0.0", Pinned: true}); len(notes) != 1 || !strings.HasPrefix(notes[0], "no-copy: ") {
		t.Errorf("notes %v", notes)
	}
	if paths := cdn.paths(); !reflect.DeepEqual(paths, []string{"/Specs/" + podShardPath("OtherKit") + "/OtherKit/1.0.0/OtherKit.podspec.json"}) {
		t.Errorf("the CDN was asked %v", paths)
	}
}

// podShardPath is the CDN's shard of a pod, a/b/c.
func podShardPath(pod string) string {
	sum := md5.Sum([]byte(pod))
	digits := hex.EncodeToString(sum[:])
	return digits[0:1] + "/" + digits[1:2] + "/" + digits[2:3]
}

// The directory of a CDN source (~/.cocoapods/repos/trunk, its .url naming the
// CDN) holds what CocoaPods downloaded: a version list or podspec found there is
// read from the disk, anything else from the CDN.
//
// Verifies: REQ-SUP-074
func TestCocoaPodsCDNSourceDirectory(t *testing.T) {
	shard := podShardPath("Cached")
	cdn := newFeed(t, map[string]string{
		"/all_pods_versions_" + strings.ReplaceAll(podShardPath("Fetched"), "/", "_") + ".txt": "Fetched/1.0.0\n",
		"/Specs/" + podShardPath("Fetched") + "/Fetched/1.0.0/Fetched.podspec.json":            `{"name":"Fetched","dependencies":{"Cached":[]}}`,
	})
	asPublic(t, CocoaPods, cdn)
	home := t.TempDir()
	trunk := filepath.Join(home, ".cocoapods", "repos", "trunk")
	put(t, filepath.Join(trunk, ".url"), cdn.URL+"\n")
	put(t, filepath.Join(trunk, "all_pods_versions_"+strings.ReplaceAll(shard, "/", "_")+".txt"), "Cached/1.0.0/1.1.0\n")
	put(t, filepath.Join(trunk, "Specs", filepath.FromSlash(shard), "Cached", "1.1.0", "Cached.podspec.json"), `{"name":"Cached","dependencies":{"Leaf":["~> 1.0"]}}`)
	c := newClient(t, NewDiscoverer(environment(nil), home).Discover(nil))
	if got, _ := ask(t, c, lang.Target{Ecosystem: CocoaPods, Package: "Cached"}); !reflect.DeepEqual(got, []string{"Leaf"}) {
		t.Errorf("Cached: %v", got)
	}
	if paths := cdn.paths(); len(paths) != 0 {
		t.Errorf("asked %v for a pod on the disk", paths)
	}
	if got, _ := ask(t, c, lang.Target{Ecosystem: CocoaPods, Package: "Fetched"}); !reflect.DeepEqual(got, []string{"Cached"}) {
		t.Errorf("Fetched: %v", got)
	}
	if paths := cdn.paths(); len(paths) != 2 {
		t.Errorf("asked %v for a pod not on the disk", paths)
	}
}

// swiftHome is a home whose registries.json maps a default registry and the
// acme scope, with a registry login in the netrc for the evil host too.
func swiftHome(t *testing.T, registry string) string {
	t.Helper()
	home := t.TempDir()
	put(t, filepath.Join(home, ".swiftpm", "configuration", "registries.json"), `{"registries":{
  "[default]":{"url":"https://registry.corp.test"},
  "acme":{"url":"`+registry+`/"}},"version":1}`)
	put(t, filepath.Join(home, ".netrc"), "machine evil.test login token password leaked\n")
	return home
}

// SwiftPM asks one registry per package: the repository's registries.json
// (beside its Package.swift, read from the disk) over the user's, scope by scope
// then default by default. A registry only the repository names is not known -
// not even when this machine holds a credential for its host - unless the user
// vouches for it or the user's own registries.json names it too. A package named
// by its repository's URL is asked of no registry.
//
// Verifies: REQ-SUP-075, REQ-SUP-043, REQ-SUP-064
func TestSwiftPMRegistriesAreScoped(t *testing.T) {
	home := swiftHome(t, "https://acme.corp.test")
	files := write(t, map[string]string{
		"Package.swift": "// swift-tools-version:5.9\nimport PackageDescription\n",
		".swiftpm/configuration/registries.json": `{"registries":{
  "[default]":{"url":"https://evil.test"},
  "mona":{"url":"https://evil.test/mona"},
  "shared":{"url":"https://acme.corp.test"}},"version":1}`,
	})
	config := NewDiscoverer(environment(nil), home).Discover(files)
	for packageName, want := range map[string][]string{
		"mona.LinkedList":   {"https://evil.test/mona?"},
		"ACME.Kit":          {"https://acme.corp.test"},
		"shared.Kit":        {"https://acme.corp.test"},
		"other.Kit":         {"https://evil.test?"},
		"github.com/a/repo": nil,
		"not-an-identity":   nil,
	} {
		if got := order(config, SwiftPM, packageName, ""); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v, want %v", packageName, got, want)
		}
	}
	config.Trust([]string{"https://evil.test/mona"})
	if index, known := config.For(SwiftPM, "mona.LinkedList"); index != "https://evil.test/mona" || !known {
		t.Errorf("vouched: %s %v", index, known)
	}
	for _, body := range []string{
		`{"registries":{"[default]":{"url":"https://x.test"},"Bad Scope":{"url":"https://y.test"}},"version":1}`,
		`{"registries":{"[default]":{"url":"https://x.test"}},"version":2}`,
	} {
		refused := t.TempDir()
		put(t, filepath.Join(refused, ".swiftpm", "configuration", "registries.json"), body)
		if got := NewDiscoverer(environment(nil), refused).Discover(nil).Sources(SwiftPM); len(got) != 0 {
			t.Errorf("a file SwiftPM refuses names %v", got)
		}
	}
	config = NewDiscoverer(environment(nil), home).Discover(nil)
	for packageName, want := range map[string][]string{"mona.LinkedList": {"https://registry.corp.test"}, "acme.Kit": {"https://acme.corp.test"}} {
		if got := order(config, SwiftPM, packageName, ""); !reflect.DeepEqual(got, want) {
			t.Errorf("the user's alone, %s: %v, want %v", packageName, got, want)
		}
	}
}

// swiftRegistry serves the Swift Package Registry API for mona.LinkedList,
// answering only to the token the netrc holds.
type swiftRegistry struct {
	*httptest.Server
	mu     sync.Mutex
	asked  []string
	accept []string
}

func newSwiftRegistry(t *testing.T) *swiftRegistry {
	t.Helper()
	r := &swiftRegistry{}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		r.mu.Lock()
		r.asked = append(r.asked, request.URL.Path)
		r.accept = append(r.accept, request.Header.Get("Accept"))
		r.mu.Unlock()
		if request.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/mona/LinkedList":
			w.Write([]byte(`{"releases":{"1.0.0":{"url":"x"},"1.1.0":{"url":"x"},"1.2.0":{"problem":{"status":410,"title":"Gone","detail":"withdrawn"}},
				"2.0.0":{"url":"x"},"1.3.0-beta.1":{"url":"x"}}}`))
		case "/mona/LinkedList/1.1.0/Package.swift":
			w.Write([]byte(`// swift-tools-version:5.9
import PackageDescription
let package = Package(name: "LinkedList", dependencies: [
    .package(id: "mona.Collections", from: "1.2.0"),
    .package(id: "acme.Exact", exact: "2.0.1"),
    .package(url: "https://github.com/apple/swift-nio.git", "2.60.0"..<"3.0.0"),
    .package(path: "../Local"),
])
`))
		case "/mona/LinkedList/2.0.0/Package.swift":
			w.Write([]byte("import PackageDescription\nlet package = Package(name: \"LinkedList\")\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(r.Close)
	return r
}

func (r *swiftRegistry) take() (asked, accept []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	asked, accept, r.asked, r.accept = r.asked, r.accept, nil, nil
	return asked, accept
}

// A registry package's releases are listed, the newest release a range admits
// that is neither withdrawn (a problem) nor a pre-release is read, and its
// Package.swift answers: registry packages by identity, repository packages by
// URL, local paths left out; an exact version is read without the listing. The
// token `swift package-registry login --token` left in the netrc goes as a
// Bearer token. A package no release admits reads the newest; one the registry
// does not have is not found.
//
// Verifies: REQ-SUP-075, REQ-AUTH-034
func TestSwiftPMRegistryDependencies(t *testing.T) {
	registry := newSwiftRegistry(t)
	home := t.TempDir()
	put(t, filepath.Join(home, ".swiftpm", "configuration", "registries.json"),
		`{"registries":{"mona":{"url":"`+registry.URL+`"}},"authentication":{"`+strings.TrimPrefix(registry.URL, "http://")+`":{"type":"token"}},"version":1}`)
	put(t, filepath.Join(home, ".netrc"), "machine 127.0.0.1 login mona password tok\n")
	config := NewDiscoverer(environment(nil), home).Discover(nil)
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, nil), nil)
	got := c.Dependencies(lang.Target{Ecosystem: SwiftPM, Package: "mona.LinkedList", Version: "1.0.0..<2.0.0"})
	want := []lang.Target{
		{Ecosystem: SwiftPM, Package: "acme.Exact", Version: "2.0.1", Pinned: true},
		{Ecosystem: SwiftPM, Package: "github.com/apple/swift-nio", Version: "2.60.0..<3.0.0"},
		{Ecosystem: SwiftPM, Package: "mona.Collections", Version: "1.2.0..<2.0.0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	asked, accept := registry.take()
	if want := []string{"/mona/LinkedList", "/mona/LinkedList/1.1.0/Package.swift"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %v, want %v", asked, want)
	}
	if want := []string{"application/vnd.swift.registry.v1+json", "application/vnd.swift.registry.v1+swift"}; !reflect.DeepEqual(accept, want) {
		t.Errorf("accept %v, want %v", accept, want)
	}
	if got := c.Dependencies(lang.Target{Ecosystem: SwiftPM, Package: "mona.LinkedList", Version: "2.0.0", Pinned: true}); len(got) != 0 {
		t.Errorf("2.0.0: %v", got)
	}
	if asked, _ := registry.take(); !reflect.DeepEqual(asked, []string{"/mona/LinkedList/2.0.0/Package.swift"}) {
		t.Errorf("an exact version: asked %v", asked)
	}
	c.Dependencies(lang.Target{Ecosystem: SwiftPM, Package: "mona.LinkedList", Version: "5.0.0..<6.0.0"})
	if asked, _ := registry.take(); !reflect.DeepEqual(asked, []string{"/mona/LinkedList", "/mona/LinkedList/2.0.0/Package.swift"}) {
		t.Errorf("no release admitted: asked %v", asked)
	}
	if _, lookup := ask(t, c, lang.Target{Ecosystem: SwiftPM, Package: "mona.Missing"}); lookup.Answer != trace.NoAnswer || !strings.Contains(lookup.Reason, "404") {
		t.Errorf("missing: %+v", lookup)
	}
	for requirement, want := range map[string]bool{"": true, "1.0.0..<2.0.0": true, "1.1.0...1.1.0": true, "1.1.0": true,
		"1.1.1": false, "1.2.0..<2.0.0": false, "branch main": false} {
		if got := swiftAdmits(requirement, "1.1.0"); got != want {
			t.Errorf("%q admits 1.1.0: %v", requirement, got)
		}
	}
}

// A registry only the repository names is not asked, whatever credential this
// machine holds for its host: the package is reported as the repository's.
//
// Verifies: REQ-SUP-075, REQ-SUP-043
func TestARepositorysSwiftRegistryIsNotAsked(t *testing.T) {
	registry := newSwiftRegistry(t)
	home := t.TempDir()
	put(t, filepath.Join(home, ".netrc"), "machine 127.0.0.1 login token password tok\n")
	files := write(t, map[string]string{
		"Package.swift":                          "import PackageDescription\n",
		".swiftpm/configuration/registries.json": `{"registries":{"[default]":{"url":"` + registry.URL + `"}},"version":1}`,
	})
	c := NewClient(NewDiscoverer(environment(nil), home).Discover(files), t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, nil), nil)
	if _, lookup := ask(t, c, lang.Target{Ecosystem: SwiftPM, Package: "mona.LinkedList"}); lookup.Reason != trace.ReasonUntrusted || lookup.Index != registry.URL {
		t.Errorf("lookup %+v", lookup)
	}
	if asked, _ := registry.take(); len(asked) != 0 {
		t.Errorf("asked %v", asked)
	}
}
