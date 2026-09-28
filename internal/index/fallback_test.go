package index

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scope"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// feed is a stub index: it serves the bodies it holds by path, answers the status
// it is told to for the paths named, and 404 for everything else, and it remembers
// every path asked of it.
type feed struct {
	*httptest.Server
	mu     sync.Mutex
	asked  []string
	status map[string]int
}

func newFeed(t *testing.T, bodies map[string]string) *feed {
	t.Helper()
	f := &feed{status: map[string]int{}}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.asked = append(f.asked, r.URL.Path)
		code := f.status[r.URL.Path]
		f.mu.Unlock()
		if code != 0 {
			w.WriteHeader(code)
			return
		}
		body, ok := bodies[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, strings.ReplaceAll(body, "{{self}}", "http://"+r.Host))
	}))
	t.Cleanup(f.Close)
	return f
}

// paths is what was asked of the feed.
func (f *feed) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.asked...)
}

// askedFor reports whether any request named the package.
func (f *feed) askedFor(packageName string) bool {
	for _, p := range f.paths() {
		if strings.Contains(p, packageName) {
			return true
		}
	}
	return false
}

// asPublic makes the feed the ecosystem's public default for the test.
func asPublic(t *testing.T, ecosystem string, f *feed) {
	t.Helper()
	was := public[ecosystem]
	public[ecosystem] = f.URL
	t.Cleanup(func() { public[ecosystem] = was })
}

// ask puts one question to a client and returns the dependency names with the
// lookup the report recorded for it.
func ask(t *testing.T, c *Client, target lang.Target) ([]string, trace.Lookup) {
	t.Helper()
	report := trace.New(1, true, nil, nil)
	c.Trace(report)
	got := names(c.Dependencies(target))
	if len(report.Lookups) != 1 {
		t.Fatalf("%s: %d lookups recorded", target.Package, len(report.Lookups))
	}
	return got, report.Lookups[0]
}

func newClient(t *testing.T, config *Config, private ...string) *Client {
	return NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil, scope.New(private))
}

func pypiJSON(dependencies ...string) string {
	return `{"info":{"requires_dist":["` + strings.Join(dependencies, `","`) + `"]}}`
}

// pip asks an extra index beside PyPI: a package that is not on the extra index is
// still found on PyPI, and one only the extra index has is found there - and is
// never named to PyPI.
//
// Verifies: REQ-SUP-063, REQ-SUP-015
func TestPipExtraIndexFallsBackToPyPI(t *testing.T) {
	pypi := newFeed(t, map[string]string{"/pypi/requests/2.31.0/json": pypiJSON("certifi")})
	corp := newFeed(t, map[string]string{"/pypi/corp-billing/1.0.0/json": pypiJSON("corp-core")})
	asPublic(t, PyPI, pypi)

	d := NewDiscoverer(environment(map[string]string{"PIP_EXTRA_INDEX_URL": corp.URL + "/simple"}), "")
	config := d.Discover(nil)
	c := newClient(t, config)

	got, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: "requests", Version: "2.31.0"})
	if len(got) != 1 || got[0] != "certifi" || l.Index != pypi.URL {
		t.Errorf("requests: %v from %s, want certifi from PyPI", got, l.Index)
	}
	got, l = ask(t, c, lang.Target{Ecosystem: PyPI, Package: "corp-billing", Version: "1.0.0"})
	if len(got) != 1 || got[0] != "corp-core" || l.Index != corp.URL+"/simple" {
		t.Errorf("corp-billing: %v from %s, want corp-core from the extra index", got, l.Index)
	}
	if pypi.askedFor("corp-billing") {
		t.Error("a package the extra index has was named to PyPI")
	}
	// The map is told where each was found.
	if index, known, ok := c.Located(PyPI, "corp-billing"); !ok || index != corp.URL+"/simple" || !known {
		t.Errorf("corp-billing located at %q (known %v, %v)", index, known, ok)
	}
	// Attributed before anything is asked, a package is PyPI's: the extra index
	// serves only what it holds.
	if index, known := config.For(PyPI, "requests"); index != pypi.URL || !known {
		t.Errorf("requests attributed to %s (known %v)", index, known)
	}
}

// A repository's extra index is not asked, but it no longer hides PyPI: every package
// PyPI has is answered by PyPI, and only one PyPI does not have is reported as
// coming from the repository's index - on the map as well as in the report.
//
// Verifies: REQ-SUP-063, REQ-SUP-018, REQ-SUP-019
func TestARepositorysExtraIndexDoesNotHidePyPI(t *testing.T) {
	pypi := newFeed(t, map[string]string{"/pypi/requests/2.31.0/json": pypiJSON("certifi")})
	corp := newFeed(t, nil)
	asPublic(t, PyPI, pypi)
	files := write(t, map[string]string{"requirements.txt": "--extra-index-url " + corp.URL + "/simple\nrequests==2.31.0\n"})
	config := Discover(files, environment(nil), "")
	c := newClient(t, config)

	if index, known := config.For(PyPI, "requests"); index != pypi.URL || !known {
		t.Errorf("requests attributed to %s (known %v), want PyPI, known", index, known)
	}
	got, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: "requests", Version: "2.31.0"})
	if len(got) != 1 || l.Answer != trace.FromIndex {
		t.Errorf("requests: %v (%+v), want PyPI's answer", got, l)
	}
	_, l = ask(t, c, lang.Target{Ecosystem: PyPI, Package: "acme-internal", Version: "1.0.0"})
	if l.Reason != trace.ReasonUntrusted || l.Index != corp.URL+"/simple" {
		t.Errorf("a package PyPI lacks: %q from %s, want the repository's index, untrusted", l.Reason, l.Index)
	}
	if index, known, ok := c.Located(PyPI, "acme-internal"); !ok || known || index != corp.URL+"/simple" {
		t.Errorf("acme-internal located at %q (known %v, %v), want the repository's index, unknown", index, known, ok)
	}
	if len(corp.paths()) != 0 {
		t.Errorf("the repository's index was asked: %v", corp.paths())
	}
}

// A package the organization owns is never named to PyPI, whatever else is
// configured: missing from the company's extra index, it is simply not found.
//
// Verifies: REQ-SUP-038, REQ-SUP-063
func TestAPrivatePackageIsNotAskedOfThePublicFallback(t *testing.T) {
	pypi := newFeed(t, nil)
	corp := newFeed(t, nil)
	asPublic(t, PyPI, pypi)
	config := NewDiscoverer(environment(map[string]string{"PIP_EXTRA_INDEX_URL": corp.URL}), "").Discover(nil)
	c := newClient(t, config, "pypi:acme-*")

	_, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: "acme-billing", Version: "1.0.0"})
	if !corp.askedFor("acme-billing") {
		t.Error("the company's extra index was not asked")
	}
	if pypi.askedFor("acme-billing") {
		t.Errorf("a private package was named to PyPI (%+v)", l)
	}
	// With nothing but PyPI to ask, it is declined, as it always was.
	only := New()
	c = newClient(t, only, "pypi:acme-*")
	if _, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: "acme-billing", Version: "1.0.0"}); l.Reason != trace.ReasonPrivate {
		t.Errorf("declined as %q, want private", l.Reason)
	}
	if len(pypi.paths()) != 0 {
		t.Errorf("PyPI was asked: %v", pypi.paths())
	}
	// The map attributes the private package to the index it would come from.
	only.Private(scope.New([]string{"pypi:acme-*"}).Match)
	only.Add(PyPI, Source{URL: "https://pypi.corp/simple", Kind: Additive, Trusted: true})
	if index, _ := only.For(PyPI, "acme-billing"); index != "https://pypi.corp/simple" {
		t.Errorf("acme-billing attributed to %s, want the company's extra index", index)
	}
}

const pomBody = `<project><dependencies><dependency><groupId>g</groupId><artifactId>%s</artifactId><version>1</version></dependency></dependencies></project>`

// A POM's repository is asked before Maven Central, which stays: an artifact the
// repository does not have is found on Central.
//
// Verifies: REQ-SUP-063, REQ-SUP-056
func TestMavenRepositoryFallsBackToCentral(t *testing.T) {
	central := newFeed(t, map[string]string{"/com/google/guava/guava/33.0/guava-33.0.pom": fmt.Sprintf(pomBody, "failureaccess")})
	nexus := newFeed(t, map[string]string{"/com/acme/billing/1.0/billing-1.0.pom": fmt.Sprintf(pomBody, "acme-core")})
	asPublic(t, Maven, central)
	files := write(t, map[string]string{"pom.xml": `<project><repositories><repository><url>` + nexus.URL + `</url></repository></repositories></project>`})

	// Only the repository names Nexus: Central answers what it has, Nexus is not
	// asked, and the artifact only Nexus could have is marked as coming from it.
	config := Discover(files, environment(nil), "")
	c := newClient(t, config)
	if got, l := ask(t, c, lang.Target{Ecosystem: Maven, Package: "com.google.guava:guava", Version: "33.0"}); len(got) != 1 || l.Index != central.URL {
		t.Errorf("guava: %v from %s, want Central's answer", got, l.Index)
	}
	if _, l := ask(t, c, lang.Target{Ecosystem: Maven, Package: "com.acme:billing", Version: "1.0"}); l.Reason != trace.ReasonUntrusted || l.Index != nexus.URL {
		t.Errorf("billing: %q from %s, want Nexus, untrusted", l.Reason, l.Index)
	}
	if len(nexus.paths()) != 0 {
		t.Errorf("an untrusted repository was asked: %v", nexus.paths())
	}

	// Vouched for, Nexus is asked first, and Central after it.
	central.mu.Lock()
	central.asked = nil
	central.mu.Unlock()
	config = Discover(files, environment(nil), "")
	config.Trust([]string{nexus.URL})
	c = newClient(t, config)
	if got, l := ask(t, c, lang.Target{Ecosystem: Maven, Package: "com.acme:billing", Version: "1.0"}); len(got) != 1 || got[0] != "g:acme-core" || l.Index != nexus.URL {
		t.Errorf("billing: %v from %s, want Nexus's answer", got, l.Index)
	}
	if central.askedFor("billing") {
		t.Error("an artifact Nexus has was named to Central")
	}
	if got, l := ask(t, c, lang.Target{Ecosystem: Maven, Package: "com.google.guava:guava", Version: "33.0"}); len(got) != 1 || l.Index != central.URL {
		t.Errorf("guava: %v from %s, want Central after Nexus", got, l.Index)
	}
	if !nexus.askedFor("guava") {
		t.Error("Nexus was not asked first")
	}
}

// A mirror of everything stands in for Central and for the repositories a POM
// declares; a mirror of Central alone leaves them beside it.
//
// Verifies: REQ-SUP-063
func TestMavenMirrorOfEverythingReplacesTheRepositories(t *testing.T) {
	for _, test := range []struct {
		mirrorOf string
		want     []string
	}{
		{"*", []string{"https://mirror.corp/maven"}},
		{"external:*", []string{"https://mirror.corp/maven"}},
		{"", []string{"https://mirror.corp/maven"}},
		{"central", []string{"https://nexus.example/repo?", "https://mirror.corp/maven"}},
		{"corp-snapshots", []string{"https://mirror.corp/maven", "https://nexus.example/repo?", "https://repo.maven.apache.org/maven2"}},
	} {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, ".m2"), 0o755); err != nil {
			t.Fatal(err)
		}
		settings := `<settings><mirrors><mirror><url>https://mirror.corp/maven</url><mirrorOf>` + test.mirrorOf + `</mirrorOf></mirror></mirrors></settings>`
		if err := os.WriteFile(filepath.Join(home, ".m2", "settings.xml"), []byte(settings), 0o644); err != nil {
			t.Fatal(err)
		}
		files := write(t, map[string]string{"pom.xml": `<project><repositories><repository><url>https://nexus.example/repo</url></repository></repositories></project>`})
		c := Discover(files, environment(nil), home)
		if got := order(c, Maven, "g:a", ""); !slices.Equal(got, test.want) {
			t.Errorf("mirrorOf %q: asked %v, want %v", test.mirrorOf, got, test.want)
		}
	}
}

// A Composer repository is asked before Packagist, which stays unless
// "packagist.org": false switches it off.
//
// Verifies: REQ-SUP-063, REQ-SUP-044
func TestComposerRepositoryFallsBackToPackagist(t *testing.T) {
	packagist := newFeed(t, map[string]string{
		"/p2/symfony/console.json": `{"packages":{"symfony/console":[{"version":"7.0.0","require":{"symfony/string":"^7"}}]}}`,
	})
	satis := newFeed(t, map[string]string{
		"/packages.json":        `{"metadata-url":"/p2/%package%.json"}`,
		"/p2/acme/billing.json": `{"packages":{"acme/billing":[{"version":"1.0.0","require":{"acme/core":"^1"}}]}}`,
	})
	asPublic(t, Composer, packagist)
	for _, off := range []bool{false, true} {
		repositories := `[{"type":"composer","url":"` + satis.URL + `"}]`
		if off {
			repositories = `[{"type":"composer","url":"` + satis.URL + `"},{"packagist.org":false}]`
		}
		config := Discover(write(t, map[string]string{"composer.json": `{"repositories":` + repositories + `}`}), environment(nil), "")
		config.Trust([]string{satis.URL})
		c := newClient(t, config)
		got, l := ask(t, c, lang.Target{Ecosystem: Composer, Package: "acme/billing"})
		if len(got) != 1 || got[0] != "acme/core" || l.Index != satis.URL {
			t.Errorf("off=%v acme/billing: %v from %s, want the repository's answer", off, got, l.Index)
		}
		got, l = ask(t, c, lang.Target{Ecosystem: Composer, Package: "symfony/console"})
		switch {
		case !off && (len(got) != 1 || l.Index != packagist.URL):
			t.Errorf("symfony/console: %v from %s, want Packagist's answer", got, l.Index)
		case off && len(got) != 0:
			t.Errorf("Packagist was switched off and still answered: %v", got)
		}
	}
	if packagist.askedFor("acme") {
		t.Error("a package the repository has was named to Packagist")
	}
	if n := len(packagist.paths()); n != 1 {
		t.Errorf("Packagist asked %d times, want once (only while it is on): %v", n, packagist.paths())
	}
}

// NuGet asks every feed it has: a package on the second feed is found there, after
// the first says it does not have it, and nuget.org is still asked last.
//
// Verifies: REQ-SUP-063, REQ-SUP-025
func TestNuGetAsksEveryFeed(t *testing.T) {
	serviceIndex := `{"resources":[{"@id":"{{self}}/flat","@type":"PackageBaseAddress/3.0.0"}]}`
	nuspec := func(dependency string) string {
		return `<package><metadata><dependencies><dependency id="` + dependency + `" version="1.0.0"/></dependencies></metadata></package>`
	}
	first := newFeed(t, map[string]string{"/index.json": serviceIndex})
	second := newFeed(t, map[string]string{
		"/index.json": serviceIndex,
		"/flat/acme.tools/1.0.0/acme.tools.nuspec": nuspec("Acme.Core"),
	})
	org := newFeed(t, map[string]string{
		"/index.json": serviceIndex,
		"/flat/newtonsoft.json/13.0.3/newtonsoft.json.nuspec": nuspec("System.Runtime"),
	})
	was := public[NuGet]
	public[NuGet] = org.URL + "/index.json"
	t.Cleanup(func() { public[NuGet] = was })

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".nuget", "NuGet"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := `<configuration><packageSources><add key="a" value="` + first.URL + `/index.json"/><add key="b" value="` + second.URL + `/index.json"/></packageSources></configuration>`
	if err := os.WriteFile(filepath.Join(home, ".nuget", "NuGet", "NuGet.Config"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newClient(t, Discover(nil, environment(nil), home))
	got, l := ask(t, c, lang.Target{Ecosystem: NuGet, Package: "Acme.Tools", Version: "1.0.0"})
	if len(got) != 1 || got[0] != "Acme.Core" || l.Index != second.URL+"/index.json" {
		t.Errorf("Acme.Tools: %v from %s, want the second feed's answer", got, l.Index)
	}
	got, l = ask(t, c, lang.Target{Ecosystem: NuGet, Package: "Newtonsoft.Json", Version: "13.0.3"})
	if len(got) != 1 || l.Index != org.URL+"/index.json" {
		t.Errorf("Newtonsoft.Json: %v from %s, want nuget.org's answer", got, l.Index)
	}
	if !first.askedFor("newtonsoft") || !second.askedFor("newtonsoft") {
		t.Error("the feeds were not asked before nuget.org")
	}
	if org.askedFor("acme") {
		t.Error("a package a feed has was named to nuget.org")
	}
	// <clear/> drops nuget.org, unless the same config names it again.
	for config, want := range map[string][]string{
		`<configuration><packageSources><clear/><add key="a" value="https://feed.corp/v3/index.json"/></packageSources></configuration>`: {
			"https://feed.corp/v3/index.json?"},
		`<configuration><packageSources><clear/><add key="a" value="https://feed.corp/v3/index.json"/>` +
			`<add key="nuget.org" value="https://api.nuget.org/v3/index.json"/></packageSources></configuration>`: {
			"https://feed.corp/v3/index.json?", org.URL + "/index.json"},
	} {
		c := Discover(write(t, map[string]string{"nuget.config": config}), environment(nil), "")
		if got := order(c, NuGet, "Acme.Tools", ""); !slices.Equal(got, want) {
			t.Errorf("asked %v, want %v", got, want)
		}
	}
}

// GOPROXY is a list the go command walks: "," moves on after 404 or 410, "|" after
// any failure; "direct" and "off" end it, and a GOPROXY that is set replaces
// proxy.golang.org altogether.
//
// Verifies: REQ-SUP-063, REQ-SUP-021
func TestGoproxyListIsWalkedAsGoDoes(t *testing.T) {
	const module = "/example.com/mod/@v/v1.0.0.mod"
	target := lang.Target{Ecosystem: Go, Package: "example.com/mod", Version: "v1.0.0"}
	for _, test := range []struct {
		name     string
		goproxy  string // A, B stand for the two stub proxies
		aStatus  int
		answered bool
		bAsked   bool
		reason   string
	}{
		{"comma after 404", "A,B", http.StatusNotFound, true, true, ""},
		{"comma after 410", "A,B,direct", http.StatusGone, true, true, ""},
		{"comma stops at 500", "A,B", http.StatusInternalServerError, false, false, "500"},
		{"pipe after 500", "A|B", http.StatusInternalServerError, true, true, ""},
		{"direct ends it", "A,direct,B", http.StatusNotFound, false, false, "404"},
		{"off ends it", "A,off", http.StatusNotFound, false, false, "404"},
		{"direct alone", "direct", 0, false, false, trace.ReasonNoIndex},
		{"off alone", "off", 0, false, false, trace.ReasonNoIndex},
	} {
		t.Run(test.name, func(t *testing.T) {
			golang := newFeed(t, map[string]string{module: "module example.com/mod\n"})
			asPublic(t, Go, golang)
			a := newFeed(t, nil)
			a.status[module] = test.aStatus
			b := newFeed(t, map[string]string{module: "module example.com/mod\n\nrequire example.com/dep v1.0.0\n"})
			goproxy := strings.NewReplacer("A", a.URL, "B", b.URL).Replace(test.goproxy)
			c := newClient(t, NewDiscoverer(environment(map[string]string{"GOPROXY": goproxy}), "").Discover(nil))
			got, l := ask(t, c, target)
			if test.answered != (len(got) == 1) {
				t.Errorf("answered %v (%+v), want answered %v", got, l, test.answered)
			}
			if test.bAsked != (len(b.paths()) > 0) {
				t.Errorf("second proxy asked: %v", b.paths())
			}
			if test.reason != "" && !strings.Contains(l.Reason, test.reason) {
				t.Errorf("reason %q, want %q", l.Reason, test.reason)
			}
			if len(golang.paths()) != 0 {
				t.Errorf("proxy.golang.org was asked although GOPROXY is set: %v", golang.paths())
			}
		})
	}
}

// A Cargo alternative registry serves the crates that declare it - by name in
// Cargo.toml or by index URL in Cargo.lock - and nothing else: every other crate is
// crates.io's, and a crates.io dependency of a registry crate goes back to it.
//
// Verifies: REQ-SUP-063, REQ-SUP-024, REQ-RS-010
func TestCargoRegistryServesOnlyTheCratesThatDeclareIt(t *testing.T) {
	crates := newFeed(t, map[string]string{"/se/rd/serde": `{"name":"serde","vers":"1.0.0","deps":[]}` + "\n"})
	corp := newFeed(t, map[string]string{
		"/bi/ll/billing": `{"name":"billing","vers":"1.0.0","deps":[{"name":"serde","req":"^1","kind":"normal","registry":"https://github.com/rust-lang/crates.io-index"},{"name":"core","req":"^1","kind":"normal"}]}` + "\n",
	})
	asPublic(t, Cargo, crates)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".cargo"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "[registries.corp]\nindex = \"sparse+" + corp.URL + "/\"\n"
	if err := os.WriteFile(filepath.Join(home, ".cargo", "config.toml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	discovered := Discover(nil, environment(nil), home)
	c := newClient(t, discovered)

	if index, known := discovered.For(Cargo, "serde"); index != crates.URL || !known {
		t.Errorf("serde attributed to %s (known %v), want crates.io", index, known)
	}
	if got, l := ask(t, c, lang.Target{Ecosystem: Cargo, Package: "serde", Version: "1.0.0"}); l.Answer != trace.FromIndex || l.Index != crates.URL || len(got) != 0 {
		t.Errorf("serde: %v %+v, want crates.io's answer", got, l)
	}
	if len(corp.paths()) != 0 {
		t.Errorf("a crates.io crate was asked of the registry: %v", corp.paths())
	}
	for _, registrySpec := range []string{"corp", "sparse+" + corp.URL + "/", "registry+" + corp.URL} {
		c := newClient(t, discovered)
		dependencies := c.Dependencies(lang.Target{Ecosystem: Cargo, Package: "billing", Version: "1.0.0", Registry: registrySpec})
		if len(dependencies) != 2 {
			t.Fatalf("registry %q: billing answered %v", registrySpec, dependencies)
		}
		for _, d := range dependencies {
			switch d.Package {
			case "serde":
				if d.Registry != "" {
					t.Errorf("serde, a crates.io crate, comes from %q", d.Registry)
				}
			case "core":
				if d.Registry != registrySpec {
					t.Errorf("core, from billing's own registry, comes from %q, want %q", d.Registry, registrySpec)
				}
			}
		}
	}
	if crates.askedFor("billing") {
		t.Error("a registry crate was named to crates.io")
	}
	// A registry nothing here configures is where the crate comes from, marked.
	if index, known := discovered.ForTarget(lang.Target{Ecosystem: Cargo, Package: "x", Registry: "registry+https://other.example/index"}); index != "https://other.example/index" || known {
		t.Errorf("an unknown registry: %s (known %v)", index, known)
	}
	// replace-with still replaces crates.io for every crate.
	discovered.Add(Cargo, Source{URL: "https://mirror.corp/crates", Trusted: true})
	if got := order(discovered, Cargo, "serde", ""); !slices.Equal(got, []string{"https://mirror.corp/crates"}) {
		t.Errorf("with a replacement: %v", got)
	}
}

// A scoped source is authoritative: a package of the scope that it does not have is
// not looked for on the public registry, where an attacker would plant it.
//
// Verifies: REQ-SUP-063, REQ-SUP-014
func TestAScopedSourceDoesNotFallBack(t *testing.T) {
	npm := newFeed(t, map[string]string{"/@acme%2fwidgets/1.0.0": `{"dependencies":{"evil":"1"}}`})
	acme := newFeed(t, nil)
	asPublic(t, NPM, npm)
	config := New()
	config.Add(NPM, Source{URL: acme.URL, Scope: "@acme", Trusted: true})
	c := newClient(t, config)

	got, l := ask(t, c, lang.Target{Ecosystem: NPM, Package: "@acme/widgets", Version: "1.0.0"})
	if len(got) != 0 || l.Index != acme.URL {
		t.Errorf("@acme/widgets: %v from %s, want nothing from the scope's registry", got, l.Index)
	}
	if !acme.askedFor("widgets") || npm.askedFor("acme") {
		t.Errorf("scope registry asked %v, public registry asked %v", acme.paths(), npm.paths())
	}
}

// Each Python tool's own words decide whether an index replaces PyPI, is asked
// beside it, or serves only the packages pinned to it.
//
// Verifies: REQ-SUP-015, REQ-SUP-063
func TestPythonIndexKinds(t *testing.T) {
	for _, test := range []struct {
		name, file, body string
		packageName      string
		want             []string
	}{
		{"pip.conf extra over two lines", "pip.conf",
			"[global]\nextra-index-url = https://a.corp/simple\n    https://b.corp/simple\n", "x",
			[]string{"https://a.corp/simple?", "https://b.corp/simple?", "https://pypi.org/simple"}},
		{"pip.conf index-url replaces", "pip.conf", "[global]\nindex-url = https://mirror.corp/simple\n", "x",
			[]string{"https://mirror.corp/simple?"}},
		{"requirements -i and extra", "requirements.txt",
			"-i https://mirror.corp/simple\n--extra-index-url=https://extra.corp/simple\n", "x",
			[]string{"https://extra.corp/simple?", "https://mirror.corp/simple?"}},
		{"poetry primary replaces", "pyproject.toml",
			"[[tool.poetry.source]]\nname = \"corp\"\nurl = \"https://poetry.corp/simple\"\n", "x",
			[]string{"https://poetry.corp/simple?"}},
		{"poetry supplemental beside", "pyproject.toml",
			"[[tool.poetry.source]]\nname = \"corp\"\nurl = \"https://poetry.corp/simple\"\npriority = \"supplemental\"\n", "x",
			[]string{"https://poetry.corp/simple?", "https://pypi.org/simple"}},
		{"poetry explicit serves its dependency", "pyproject.toml",
			"[tool.poetry.dependencies]\nacme = { version = \"^1\", source = \"corp\" }\n\n" +
				"[[tool.poetry.source]]\nname = \"corp\"\nurl = \"https://poetry.corp/simple\"\npriority = \"explicit\"\n", "acme",
			[]string{"https://poetry.corp/simple?"}},
		{"poetry explicit serves nothing else", "pyproject.toml",
			"[tool.poetry.dependencies]\nacme = { version = \"^1\", source = \"corp\" }\n\n" +
				"[[tool.poetry.source]]\nname = \"corp\"\nurl = \"https://poetry.corp/simple\"\npriority = \"explicit\"\n", "requests",
			[]string{"https://pypi.org/simple"}},
		{"uv index beside", "pyproject.toml", "[[tool.uv.index]]\nname = \"corp\"\nurl = \"https://uv.corp/simple\"\n", "x",
			[]string{"https://uv.corp/simple?", "https://pypi.org/simple"}},
		{"uv default replaces", "pyproject.toml",
			"[[tool.uv.index]]\nname = \"corp\"\nurl = \"https://uv.corp/simple\"\ndefault = true\n", "x",
			[]string{"https://uv.corp/simple?"}},
		{"uv explicit pinned", "pyproject.toml",
			"[tool.uv.sources]\ntorch = { index = \"pytorch\" }\n\n[[tool.uv.index]]\nname = \"pytorch\"\n" +
				"url = \"https://download.pytorch.org/whl/cpu\"\nexplicit = true\n", "torch",
			[]string{"https://download.pytorch.org/whl/cpu?"}},
	} {
		c := Discover(write(t, map[string]string{test.file: test.body}), environment(nil), "")
		if got := order(c, PyPI, test.packageName, ""); !slices.Equal(got, test.want) {
			t.Errorf("%s: asked %v, want %v", test.name, got, test.want)
		}
	}
}

// Packagist is switched off in the object form of "repositories" too, and under
// --watch what the repository switched off comes back when it stops saying so.
//
// Verifies: REQ-SUP-063, REQ-SUP-015
func TestPackagistSwitchedOffIsForgottenWithTheRepository(t *testing.T) {
	d := NewDiscoverer(environment(nil), "")
	files := write(t, map[string]string{"composer.json": `{"repositories": {"packagist.org": false}}`})
	if got := order(d.Discover(files), Composer, "a/b", ""); len(got) != 0 {
		t.Errorf("Packagist switched off, still asked: %v", got)
	}
	if err := os.WriteFile(files[0].AbsolutePath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := order(d.Discover(files), Composer, "a/b", ""); !slices.Equal(got, []string{"https://repo.packagist.org"}) {
		t.Errorf("after the repository stopped switching it off: %v", got)
	}
}
