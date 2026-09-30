package index

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scope"
	"github.com/sarumaj/depphunter-cli/internal/trace"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// stubRequests is what a stub was asked, safe for the client's goroutines.
type stubRequests struct {
	mu   sync.Mutex
	seen []string
}

func (s *stubRequests) add(r string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen = append(s.seen, r)
}

func (s *stubRequests) take() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.seen
	s.seen = nil
	return out
}

// Verifies: REQ-SUP-069, REQ-SUP-071
func TestRangeAdmits(t *testing.T) {
	for _, c := range []struct {
		requirement, version string
		bareCaret, want      bool
	}{
		{">= 4.13.1 < 10.0.0", "9.6.0", false, true},
		{">= 4.13.1 < 10.0.0", "10.0.0", false, false},
		{">=4.13.1 <10.0.0", "4.13.0", false, false},
		{"1.x", "1.9.9", false, true},
		{"1.x", "2.0.0", false, false},
		{"1.2.x", "1.3.0", false, false},
		{"1.2.3", "1.2.3", false, true},
		{"1.2.3", "1.2.4", false, false},
		{"1.2.3", "1.9.0", true, true}, // Wally: a bare version is a caret range
		{"1.2.3", "2.0.0", true, false},
		{"0.2.3", "0.3.0", true, false},
		{">=1.0.0, <2.0.0", "1.5.0", true, true},
		{">=1.0.0, <2.0.0", "2.0.0", true, false},
		{"=1.2.3", "1.2.3", true, true},
		{"=1.2.3", "1.2.4", true, false},
		{"~1.2.3", "1.2.9", false, true},
		{"~1.2.3", "1.3.0", false, false},
		{"~> 1.2", "1.9.0", false, true},
		{"~> 1.2.3", "1.3.0", false, false},
		{"^0.0.3", "0.0.4", false, false},
		{"1.0.0 - 2.0.0", "2.0.0", false, true},
		{"1.0.0 - 2.0.0", "2.0.1", false, false},
		{"< 1.0.0 || >= 3.0.0", "3.1.0", false, true},
		{"< 1.0.0 || >= 3.0.0", "2.0.0", false, false},
		{">1.2", "1.2.9", false, false},
		{"<=1.2", "1.2.9", false, true},
		{"*", "7.0.0", false, true},
		{"", "7.0.0", true, true},
		{"banana", "1.0.0", false, false},
	} {
		v, _, ok := semanticVersion(c.version)
		if !ok {
			t.Fatalf("%s is not a version", c.version)
		}
		if got := rangeAdmits(c.requirement, v, c.bareCaret); got != c.want {
			t.Errorf("rangeAdmits(%q, %s, caret %v) = %v, want %v", c.requirement, c.version, c.bareCaret, got, c.want)
		}
	}
	if got := newestAdmitted([]string{"1.0.0", "1.4.0", "1.5.0-rc.1", "2.0.0"}, "1.x", false); got != "1.4.0" {
		t.Errorf("newest 1.x = %q", got)
	}
	if got := newestAdmitted([]string{"1.0.0", "1.5.0-rc.1"}, "1.5.0-rc.1", false); got != "1.5.0-rc.1" {
		t.Errorf("an exact pre-release = %q", got)
	}
}

// The Forge's v3 API: an exact version is its release's metadata.json; a range is
// the newest release the module lists that it admits, deleted releases and
// pre-releases left out, the current release's metadata used when it is the one.
//
// Verifies: REQ-SUP-069
func TestForgeDependencies(t *testing.T) {
	var asked stubRequests
	release := func(version string, dependencies ...string) string {
		var list []string
		for _, d := range dependencies {
			name, requirement, _ := strings.Cut(d, "@")
			list = append(list, `{"name":"`+name+`","version_requirement":"`+requirement+`"}`)
		}
		return `{"slug":"puppetlabs-apache-` + version + `","version":"` + version + `","metadata":{"name":"puppetlabs-apache","dependencies":[` + strings.Join(list, ",") + `]}}`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.add(r.URL.Path)
		switch r.URL.Path {
		case "/v3/releases/puppetlabs-apache-12.0.0":
			io.WriteString(w, release("12.0.0", "puppetlabs/stdlib@>= 9.0.0 < 10.0.0", "puppetlabs/concat@>= 2.2.1 < 10.0.0"))
		case "/v3/releases/puppetlabs-apache-11.1.0":
			io.WriteString(w, release("11.1.0", "puppetlabs/stdlib@>= 8.0.0 < 10.0.0"))
		case "/v3/modules/puppetlabs-apache":
			io.WriteString(w, `{"slug":"puppetlabs-apache","current_release":`+release("12.0.0", "puppetlabs/stdlib@>= 9.0.0 < 10.0.0", "puppetlabs/concat@>= 2.2.1 < 10.0.0")+
				`,"releases":[{"version":"12.1.0-rc0","deleted_at":null},{"version":"12.0.0","deleted_at":null},{"version":"11.2.0","deleted_at":"2024-01-01 00:00:00 -0700"},{"version":"11.1.0","deleted_at":null}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := clientFor(t, PuppetForge, server.URL, "")
	want12 := []lang.Target{
		{Ecosystem: PuppetForge, Package: "puppetlabs-concat", Version: ">= 2.2.1 < 10.0.0"},
		{Ecosystem: PuppetForge, Package: "puppetlabs-stdlib", Version: ">= 9.0.0 < 10.0.0"},
	}
	if got := c.Dependencies(lang.Target{Ecosystem: PuppetForge, Package: "puppetlabs-apache", Version: "12.0.0", Pinned: true}); !reflect.DeepEqual(got, want12) {
		t.Errorf("12.0.0: got %+v, want %+v", got, want12)
	}
	if got := asked.take(); !reflect.DeepEqual(got, []string{"/v3/releases/puppetlabs-apache-12.0.0"}) {
		t.Errorf("asked %v", got)
	}
	// The newest the range admits is the current release: one request.
	if got := c.Dependencies(lang.Target{Ecosystem: PuppetForge, Package: "puppetlabs/apache", Version: ">= 11.0.0 < 13.0.0"}); !reflect.DeepEqual(got, want12) {
		t.Errorf("range: got %+v", got)
	}
	if got := asked.take(); !reflect.DeepEqual(got, []string{"/v3/modules/puppetlabs-apache"}) {
		t.Errorf("asked %v", got)
	}
	// 11.x: 11.2.0 was deleted, so 11.1.0.
	want11 := []lang.Target{{Ecosystem: PuppetForge, Package: "puppetlabs-stdlib", Version: ">= 8.0.0 < 10.0.0"}}
	if got := c.Dependencies(lang.Target{Ecosystem: PuppetForge, Package: "puppetlabs-apache", Version: "11.x"}); !reflect.DeepEqual(got, want11) {
		t.Errorf("11.x: got %+v", got)
	}
	if got := asked.take(); !reflect.DeepEqual(got, []string{"/v3/modules/puppetlabs-apache", "/v3/releases/puppetlabs-apache-11.1.0"}) {
		t.Errorf("asked %v", got)
	}
	// Not a slug (a git module named by its repository): nothing is asked.
	if got := c.Dependencies(lang.Target{Ecosystem: PuppetForge, Package: "github.com/x/y"}); got != nil || len(asked.take()) != 0 {
		t.Errorf("not a slug: %+v", got)
	}
	if index, known := New().For(PuppetForge, "puppetlabs-apache"); index != "https://forgeapi.puppet.com" || !known {
		t.Errorf("public index %q %v", index, known)
	}
}

// A Puppetfile's `forge` line is the repository's (the public Forge's own hosts are
// not recorded); r10k.yaml's forge.baseurl is this machine's.
//
// Verifies: REQ-SUP-069, REQ-SUP-064
func TestDiscoverForges(t *testing.T) {
	files := write(t, map[string]string{
		"Puppetfile":          "forge 'https://forgeapi.puppetlabs.com'\nmod 'puppetlabs-stdlib', '9.6.0'\n",
		"site/Puppetfile":     "forge \"forge.corp.example/api/\"\n",
		"modules/x/README.md": "forge 'https://ignored.example'\n",
	})
	c := Discover(files, environment(nil), "")
	if got := order(c, PuppetForge, "puppetlabs-stdlib", ""); !reflect.DeepEqual(got, []string{"https://forge.corp.example/api?"}) {
		t.Errorf("repository forge: %v", got)
	}
	saved := userconf.SystemRoot
	userconf.SystemRoot = t.TempDir()
	t.Cleanup(func() { userconf.SystemRoot = saved })
	put(t, filepath.Join(userconf.SystemRoot, "etc", "puppetlabs", "r10k", "r10k.yaml"),
		"forge:\n  baseurl: https://forge.machine.example\n  authorization_token: 'Bearer secret'\n")
	put(t, filepath.Join(userconf.SystemRoot, "etc", "r10k.yaml"), "forge:\n  baseurl: https://older.example\n")
	c = Discover(files, environment(nil), t.TempDir())
	if got := order(c, PuppetForge, "puppetlabs-stdlib", ""); !reflect.DeepEqual(got, []string{"https://forge.machine.example"}) {
		t.Errorf("machine forge: %v", got)
	}
}

// A catalog entry is a hash table; its dependencies have info.rkt's shape, and
// "base" is the base collections. A version-specific catalog's default entry is
// read when the table has no dependencies of its own; an answer that is not a hash
// table is the catalog not having the package.
//
// Verifies: REQ-SUP-070, REQ-RACKET-012
func TestRacketCatalogDependencies(t *testing.T) {
	var asked stubRequests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.add(r.URL.Path)
		switch r.URL.Path {
		case "/pkg/rebellion":
			io.WriteString(w, `#hash((name . "rebellion") (source . "git://github.com/jackfirth/rebellion") (checksum . "abc")
 (dependencies . ("base" ("rackunit-lib" #:version "1.2") "https://github.com/x/fancy-lib.git#main"))
 (tags . ("data")))`)
		case "/pkg/versioned":
			io.WriteString(w, `#hasheq((name . "versioned") (versions . #hash((default . #hash((dependencies . ("typed-racket-lib")))) ("6.0" . #hash((dependencies . ("old")))))))`)
		case "/pkg/missing":
			io.WriteString(w, "#f")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := clientFor(t, Racket, server.URL, "")
	got := c.Dependencies(lang.Target{Ecosystem: Racket, Package: "rebellion", Floating: true})
	want := []lang.Target{
		{Ecosystem: "racket-std", Package: "base"},
		{Ecosystem: Racket, Package: "rackunit-lib", Version: "1.2"},
		{Ecosystem: Racket, Package: "fancy-lib", Version: "main"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := names(c.Dependencies(lang.Target{Ecosystem: Racket, Package: "versioned"})); !reflect.DeepEqual(got, []string{"typed-racket-lib"}) {
		t.Errorf("versions default: %v", got)
	}
	_, l := ask(t, c, lang.Target{Ecosystem: Racket, Package: "missing"})
	if l.Answer != trace.NoAnswer || !strings.Contains(l.Reason, "does not list") {
		t.Errorf("missing: %+v", l)
	}
	asked.take()
	if got := c.Dependencies(lang.Target{Ecosystem: Racket, Package: "../x"}); got != nil || len(asked.take()) != 0 {
		t.Errorf("bad name asked: %+v", got)
	}
	if index, known := New().For(Racket, "rebellion"); index != "https://pkgs.racket-lang.org" || !known {
		t.Errorf("public index %q %v", index, known)
	}
}

// wallyStub serves GitHub repositories' files: path -> body.
func wallyStub(t *testing.T, files map[string]string) *stubRequests {
	t.Helper()
	var asked stubRequests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.add(r.URL.Path)
		if body, ok := files[r.URL.Path]; ok {
			io.WriteString(w, body)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	was := githubRaw
	githubRaw = server.URL
	t.Cleanup(func() { githubRaw = was })
	return &asked
}

func wallyLine(name, version, registry string, dependencies, server, dev map[string]string) string {
	line, _ := json.Marshal(map[string]any{
		"package":             map[string]string{"name": name, "version": version, "registry": registry, "realm": "shared"},
		"dependencies":        dependencies,
		"server-dependencies": server,
		"dev-dependencies":    dev,
	})
	return string(line) + "\n"
}

// A Wally registry is a GitHub repository of package files, one manifest per line:
// the version asked for (a bare version is a caret range) gives its dependencies
// and server-dependencies, each from the registry its manifest names; a package
// the registry lacks is looked for in the fallback registries its config.json
// lists.
//
// Verifies: REQ-SUP-071
func TestWallyDependencies(t *testing.T) {
	public := "https://github.com/UpliftGames/wally-index"
	corp := "https://github.com/corp/wally-index"
	asked := wallyStub(t, map[string]string{
		"/UpliftGames/wally-index/HEAD/sleitnick/knit": wallyLine("sleitnick/knit", "1.6.0", public, map[string]string{"Promise": "evaera/promise@>=4.0.0, <5.0.0"}, nil, nil) +
			wallyLine("sleitnick/knit", "1.7.0", public, map[string]string{"Comm": "sleitnick/comm@>=1.0.0, <2.0.0", "Promise": "evaera/promise@>=4.0.0, <5.0.0"},
				map[string]string{"Data": "corp/datastore@1.0.0"}, map[string]string{"TestEZ": "roblox/testez@0.4.1"}) +
			wallyLine("sleitnick/knit", "2.0.0-rc.1", public, nil, nil, nil),
		"/UpliftGames/wally-index/HEAD/config.json": `{"api": "https://api.wally.run/", "fallback_registries": ["https://github.com/corp/wally-index"]}`,
		"/corp/wally-index/HEAD/corp/tools":         wallyLine("corp/tools", "0.3.1", corp, map[string]string{"Log": "corp/log@0.1.0"}, nil, nil),
	})
	c := NewClient(Discover(nil, environment(nil), ""), t.TempDir(), time.Hour, 5*time.Second, nil, nil)
	want := []lang.Target{
		{Ecosystem: Wally, Package: "sleitnick/comm", Version: ">=1.0.0, <2.0.0", Registry: public},
		{Ecosystem: Wally, Package: "evaera/promise", Version: ">=4.0.0, <5.0.0", Registry: public},
		{Ecosystem: Wally, Package: "corp/datastore", Version: "1.0.0", Registry: public},
	}
	if got := c.Dependencies(lang.Target{Ecosystem: Wally, Package: "sleitnick/knit", Version: "1.2.0"}); !reflect.DeepEqual(got, want) {
		t.Errorf("^1.2.0: got %+v, want %+v", got, want)
	}
	if got := asked.take(); !reflect.DeepEqual(got, []string{"/UpliftGames/wally-index/HEAD/sleitnick/knit"}) {
		t.Errorf("asked %v", got)
	}
	if got := names(c.Dependencies(lang.Target{Ecosystem: Wally, Package: "sleitnick/knit", Version: "1.6.0", Pinned: true})); !reflect.DeepEqual(got, []string{"evaera/promise"}) {
		t.Errorf("1.6.0: %v", got)
	}
	// Not on the registry: its fallback has it, and names itself for the dependencies.
	asked.take()
	got := c.Dependencies(lang.Target{Ecosystem: Wally, Package: "corp/tools", Version: "0.3.1", Pinned: true, Registry: public})
	if want := []lang.Target{{Ecosystem: Wally, Package: "corp/log", Version: "0.1.0", Registry: corp}}; !reflect.DeepEqual(got, want) {
		t.Errorf("fallback: got %+v, want %+v", got, want)
	}
	if want := []string{"/UpliftGames/wally-index/HEAD/corp/tools", "/UpliftGames/wally-index/HEAD/config.json", "/corp/wally-index/HEAD/corp/tools"}; !reflect.DeepEqual(asked.take(), want) {
		t.Error("fallback not asked in order")
	}
	// A dependency's registry nobody vouches for is not asked.
	_, l := ask(t, c, lang.Target{Ecosystem: Wally, Package: "corp/log", Version: "0.1.0", Registry: corp + ".git"})
	if l.Reason != trace.ReasonUntrusted || l.Index != corp {
		t.Errorf("corp registry: %+v", l)
	}
	if len(asked.take()) != 0 {
		t.Error("an untrusted registry was asked")
	}
	// wally.toml names the registry; the public one is known, another is not.
	files := write(t, map[string]string{
		"game/wally.toml": "[package]\nname = \"me/game\"\nversion = \"0.1.0\"\nregistry = \"https://github.com/corp/wally-index.git\"\nrealm = \"shared\"\n",
	})
	if got := order(Discover(files, environment(nil), ""), Wally, "sleitnick/knit", ""); !reflect.DeepEqual(got, []string{corp + "?"}) {
		t.Errorf("wally.toml registry: %v", got)
	}
	if index, known := New().For(Wally, "sleitnick/knit"); index != public || !known {
		t.Errorf("public index %q %v", index, known)
	}
	// Only GitHub registries are read.
	if _, err := wallyFiles("https://gitlab.com/corp/index"); err == nil {
		t.Error("a GitLab registry was given a file URL")
	}
}

// bufStub is a Buf Schema Registry answering the three calls of one graph:
// acme/payments at a label depends on googleapis and protovalidate, protovalidate
// on googleapis, and on one module of another registry.
func bufStub(t *testing.T) (*httptest.Server, *stubRequests) {
	t.Helper()
	var asked stubRequests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		asked.add(r.Method + " " + r.URL.Path + " " + r.Header.Get("Authorization") + " " + strings.TrimSpace(string(body)))
		if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Connect-Protocol-Version") != "1" {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		switch r.URL.Path {
		case "/buf.registry.module.v1.GraphService/GetGraph":
			if !strings.Contains(string(body), `"module":"payments"`) {
				w.WriteHeader(http.StatusNotFound)
				io.WriteString(w, `{"code":"not_found","message":"module not found"}`)
				return
			}
			io.WriteString(w, `{"graph":{"commits":[
{"id":"a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1","ownerId":"o1","moduleId":"m1"},
{"id":"b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2","ownerId":"o2","moduleId":"m2"},
{"id":"c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3","ownerId":"o3","moduleId":"m3"},
{"id":"d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4","ownerId":"o9","moduleId":"m9"}],
"edges":[
{"fromNode":{"commitId":"a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1"},"toNode":{"commitId":"b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2"}},
{"fromNode":{"commitId":"a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1"},"toNode":{"commitId":"c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3"}},
{"fromNode":{"commitId":"c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3"},"toNode":{"commitId":"b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2"}},
{"fromNode":{"commitId":"a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1"},"toNode":{"commitId":"d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4"}}],
"registryCommitIds":[{"commitId":"d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4","registry":"buf.other.example"}]}}`)
		case "/buf.registry.module.v1.ModuleService/GetModules":
			io.WriteString(w, `{"modules":[{"id":"m2","name":"googleapis","ownerId":"o2"},{"id":"m3","name":"protovalidate","ownerId":"o3"}]}`)
		case "/buf.registry.owner.v1.OwnerService/GetOwners":
			io.WriteString(w, `{"owners":[{"organization":{"id":"o2","name":"googleapis"}},{"user":{"id":"o3","name":"bufbuild"}}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server, &asked
}

// A module's direct dependencies are the commits the graph's edges lead to from
// the commit asked about, named through the module and owner services and pinned
// by commit; a dependency on another registry is left out. The token is the one
// BUF_TOKEN names for the registry's host, and a remote plugin is not asked.
//
// Verifies: REQ-SUP-072, REQ-AUTH-031, REQ-TRC-006, REQ-SUP-043
func TestBufModuleDependencies(t *testing.T) {
	server, asked := bufStub(t)
	was := public[Buf]
	public[Buf] = server.URL
	t.Cleanup(func() { public[Buf] = was })
	host := Host(server.URL)
	credentials := auth.Read(t.TempDir(), environment(map[string]string{"BUF_TOKEN": "secret@" + host + ",other@buf.corp.example"}))
	c := NewClient(New(), t.TempDir(), time.Hour, 5*time.Second, credentials, nil)
	got := c.Dependencies(lang.Target{Ecosystem: Buf, Package: host + "/acme/payments", Version: "v1.2.0"})
	want := []lang.Target{
		{Ecosystem: Buf, Package: host + "/bufbuild/protovalidate", Version: "c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3", Pinned: true},
		{Ecosystem: Buf, Package: host + "/googleapis/googleapis", Version: "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2", Pinned: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	wantAsked := []string{
		`POST /buf.registry.module.v1.GraphService/GetGraph Bearer secret {"resourceRefs":[{"name":{"module":"payments","owner":"acme","ref":"v1.2.0"}}]}`,
		`POST /buf.registry.module.v1.ModuleService/GetModules Bearer secret {"moduleRefs":[{"id":"m2"},{"id":"m3"}]}`,
		`POST /buf.registry.owner.v1.OwnerService/GetOwners Bearer secret {"ownerRefs":[{"id":"o2"},{"id":"o3"}]}`,
	}
	if got := asked.take(); !reflect.DeepEqual(got, wantAsked) {
		t.Errorf("asked\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantAsked, "\n"))
	}
	// A module the registry does not have: not found, one request.
	_, l := ask(t, c, lang.Target{Ecosystem: Buf, Package: host + "/acme/missing"})
	if l.Answer != trace.NoAnswer || !strings.Contains(l.Reason, "404") || len(asked.take()) != 1 {
		t.Errorf("missing: %+v", l)
	}
	// A remote plugin is not asked.
	_, l = ask(t, c, lang.Target{Ecosystem: Buf, Package: host + "/protocolbuffers/go", Version: "v1.34.1", Registry: BufPlugin})
	if l.Reason != trace.ReasonPlugin || len(asked.take()) != 0 {
		t.Errorf("plugin: %+v", l)
	}
	// Another registry's module is asked of that host, known only when buf's
	// credentials name it (or the user vouches for it).
	config := New()
	config.Credentials(credentials)
	if got := order(config, Buf, "buf.corp.example/acme/x", ""); !reflect.DeepEqual(got, []string{"https://buf.corp.example"}) {
		t.Errorf("credentialed registry: %v", got)
	}
	if got := order(config, Buf, "buf.else.example/acme/x", ""); !reflect.DeepEqual(got, []string{"https://buf.else.example?"}) {
		t.Errorf("unknown registry: %v", got)
	}
	config.Trust([]string{"https://buf.else.example"})
	if got := order(config, Buf, "buf.else.example/acme/x", ""); !reflect.DeepEqual(got, []string{"https://buf.else.example"}) {
		t.Errorf("vouched registry: %v", got)
	}
	public[Buf] = was
	if index, known := New().For(Buf, "buf.build/googleapis/googleapis"); index != "https://buf.build" || !known {
		t.Errorf("public index %q %v", index, known)
	}
}

// plainHosts are hosts a credential is asked to reach over plain http: this
// machine under names net/http sends past a proxy, other spellings of it that
// it does not, and hosts elsewhere.
var plainHosts = []string{
	"localhost", "localhost:4873", "127.0.0.1", "127.0.0.2:8080", "[::1]:5000", "[::ffff:127.0.0.1]",
	"LOCALHOST", "localhost.", "127.1", "[fe80::1%25lo0]", "10.0.0.1", "registry.corp.example",
}

// sendsCredential reports, for each of plainHosts, whether Apply sends this
// machine's credential for the host over plain http (the machine names none of
// them with http://), so that the paths that send a credential of their own can
// be held to the same set.
func sendsCredential(t *testing.T) map[string]bool {
	t.Helper()
	home := t.TempDir()
	var netrc strings.Builder
	for _, host := range plainHosts {
		netrc.WriteString("machine " + Host("http://"+host) + " login user password secret\n")
	}
	put(t, filepath.Join(home, ".netrc"), netrc.String())
	credentials := auth.Read(home, environment(nil))
	out := map[string]bool{}
	for _, host := range plainHosts {
		out[host] = credentials.Authorizes("http://" + host + "/x")
	}
	for _, host := range []string{"127.0.0.2:8080", "[::ffff:127.0.0.1]"} {
		if !out[host] {
			t.Fatalf("Apply does not send a credential to %s", host)
		}
	}
	for _, host := range []string{"LOCALHOST", "10.0.0.1"} {
		if out[host] {
			t.Fatalf("Apply sends a credential to %s", host)
		}
	}
	return out
}

// authorizations is a transport that records the Authorization header of every
// request by path and answers with answer.
type authorizations struct {
	mu     sync.Mutex
	sent   map[string]string
	answer func(*http.Request) (int, string)
}

func (a *authorizations) RoundTrip(r *http.Request) (*http.Response, error) {
	a.mu.Lock()
	a.sent[r.URL.Path] = r.Header.Get("Authorization")
	a.mu.Unlock()
	code, body := a.answer(r)
	return &http.Response{StatusCode: code, Status: http.StatusText(code), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

// A Buf token goes over plain http to exactly the hosts Apply sends a
// credential to: this machine as auth defines it, and nowhere else.
//
// Verifies: REQ-AUTH-031, REQ-AUTH-011
func TestBufTokenGoesWhereApplySendsACredential(t *testing.T) {
	want := sendsCredential(t)
	credentials := auth.Read(t.TempDir(), environment(map[string]string{"BUF_TOKEN": "token@buf.test"}))
	for _, host := range plainHosts {
		transport := &authorizations{sent: map[string]string{}, answer: func(*http.Request) (int, string) { return http.StatusOK, "{}" }}
		c := NewClient(New(), t.TempDir(), time.Hour, 5*time.Second, credentials, nil)
		c.http = &http.Client{Transport: transport}
		var reply struct{}
		if err := c.bufCall(t.Context(), "http://"+host, "buf.test", "buf.registry.module.v1.GraphService/GetGraph", map[string]any{}, &reply); err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		sent := transport.sent["/buf.registry.module.v1.GraphService/GetGraph"] == "Bearer token"
		if sent != want[host] {
			t.Errorf("token sent to http://%s: %v, Apply sends a credential: %v", host, sent, want[host])
		}
	}
}

// cueStub is an OCI registry holding one CUE module version under a repository
// prefix, answering only with the token cue login stored.
func cueStub(t *testing.T) (*httptest.Server, *stubRequests) {
	t.Helper()
	var asked stubRequests
	moduleFile := "module: \"example.com/lib@v0\"\nlanguage: version: \"v0.9.0\"\ndeps: {\n\t\"github.com/acme/schemas@v1\": v: \"v1.2.0\"\n\t\"cue.dev/x/k8s.io@v0\": v: \"v0.4.0\"\n}\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.add(r.URL.Path + " " + r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") != "Bearer cue-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v2/modules/example.com/lib/manifests/v0.2.0":
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			io.WriteString(w, `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","artifactType":"application/vnd.cue.module.v1+json",
"config":{"mediaType":"application/vnd.cue.module.v1+json","digest":"sha256:c0"},
"layers":[{"mediaType":"application/zip","digest":"sha256:z1","size":9000},{"mediaType":"application/vnd.cue.modulefile.v1","digest":"sha256:f2","size":120}]}`)
		case "/v2/modules/example.com/lib/blobs/sha256:f2":
			io.WriteString(w, moduleFile)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, &asked
}

// A CUE module version's module.cue is read from its manifest's module file layer,
// not its archive, at the registry CUE_REGISTRY routes the module's prefix to,
// with the token `cue login` stored for that host. A module without a version is
// not asked.
//
// Verifies: REQ-SUP-073, REQ-AUTH-032, REQ-CUE-012
func TestCUEModuleDependencies(t *testing.T) {
	server, asked := cueStub(t)
	host := Host(server.URL)
	home := t.TempDir()
	configDirectory := filepath.Join(home, "cue-config")
	put(t, filepath.Join(configDirectory, "logins.json"), `{"registries":{"`+host+`":{"access_token":"cue-token","token_type":"Bearer"}}}`)
	variables := map[string]string{"CUE_REGISTRY": "example.com=" + host + "/modules+insecure,registry.corp.example", "CUE_CONFIG_DIR": configDirectory}
	config := discoverOn(home, "linux", variables)
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, environment(variables)), scope.New(nil))
	got := c.Dependencies(lang.Target{Ecosystem: CUE, Package: "example.com/lib", Version: "v0.2.0", Pinned: true})
	want := []lang.Target{
		{Ecosystem: CUE, Package: "cue.dev/x/k8s.io", Version: "v0.4.0", Pinned: true},
		{Ecosystem: CUE, Package: "github.com/acme/schemas", Version: "v1.2.0", Pinned: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if want := []string{"/v2/modules/example.com/lib/manifests/v0.2.0 Bearer cue-token", "/v2/modules/example.com/lib/blobs/sha256:f2 Bearer cue-token"}; !reflect.DeepEqual(asked.take(), want) {
		t.Error("the module file was not read alone, with the login")
	}
	_, l := ask(t, c, lang.Target{Ecosystem: CUE, Package: "example.com/other", Floating: true})
	if l.Reason != trace.ReasonNoVersion || len(asked.take()) != 0 {
		t.Errorf("unversioned: %+v", l)
	}
	if got := order(config, CUE, "github.com/acme/schemas", ""); !reflect.DeepEqual(got, []string{"https://registry.corp.example"}) {
		t.Errorf("catch-all: %v", got)
	}
	if index, known := New().For(CUE, "github.com/acme/schemas"); index != "https://registry.cue.works" || !known {
		t.Errorf("public index %q %v", index, known)
	}
}

// CUE_REGISTRY: a catch-all replaces registry.cue.works (none switches it off), a
// module prefix routes the modules under it (whole path elements, the longest
// prefix first), +insecure and this machine are plain http, a file: or inline:
// configuration is not read.
//
// Verifies: REQ-SUP-073
func TestParseCUERegistry(t *testing.T) {
	read := func(value string) *Config {
		return discoverOn(t.TempDir(), "linux", map[string]string{"CUE_REGISTRY": value})
	}
	c := read("corp.example=reg.corp.example/cue, corp.example/team=localhost:5000,reg.example+insecure")
	for packageName, want := range map[string]string{
		"corp.example/lib":       "https://reg.corp.example/cue",
		"corp.example/team/x":    "http://localhost:5000",
		"corp.example/teamwork":  "https://reg.corp.example/cue",
		"corp.examples/anything": "http://reg.example",
	} {
		if got := order(c, CUE, packageName, ""); !reflect.DeepEqual(got, []string{want}) {
			t.Errorf("%s: %v, want %s", packageName, got, want)
		}
	}
	if got := order(read("none"), CUE, "github.com/x/y", ""); len(got) != 0 {
		t.Errorf("none: %v", got)
	}
	if got := order(read("file:/etc/cue/registry.cue"), CUE, "github.com/x/y", ""); !reflect.DeepEqual(got, []string{"https://registry.cue.works"}) {
		t.Errorf("file: %v", got)
	}
	if got := order(read("simple:reg.example"), CUE, "github.com/x/y", ""); !reflect.DeepEqual(got, []string{"https://reg.example"}) {
		t.Errorf("simple: %v", got)
	}
}

// The machine files cue and r10k keep are found where the tools look.
//
// Verifies: REQ-SUP-064
func TestCUEAndR10KLocations(t *testing.T) {
	m := userconf.Machine{Home: "home", GOOS: "linux", Environment: environment(nil)}
	if got := m.CUEConfigDirectory(); got != filepath.Join("home", ".config", "cue") {
		t.Errorf("cue: %s", got)
	}
	m.Environment = environment(map[string]string{"CUE_CONFIG_DIR": "elsewhere"})
	if got := m.CUEConfigDirectory(); got != "elsewhere" {
		t.Errorf("CUE_CONFIG_DIR: %s", got)
	}
	if got := m.R10KConfigs(); len(got) != 2 || !strings.HasSuffix(got[0], filepath.Join("puppetlabs", "r10k", "r10k.yaml")) {
		t.Errorf("r10k: %v", got)
	}
	if _, err := os.Stat(m.R10KConfigs()[0]); err == nil {
		t.Error("the test machine's r10k.yaml leaked in")
	}
}
