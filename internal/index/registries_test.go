package index

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Verifies: REQ-SUP-024
func TestCargoIndex(t *testing.T) {
	for configured, want := range map[string]string{
		"":                                "https://index.crates.io",
		"https://crates.io":               "https://index.crates.io",
		"sparse+https://index.crates.io/": "https://index.crates.io",
		"https://mirror.example/cargo/":   "https://mirror.example/cargo",
	} {
		if got := cargoIndex(configured); got != want {
			t.Errorf("cargoIndex(%q) = %q, want %q", configured, got, want)
		}
	}
}

// Verifies: REQ-SUP-024
func TestCargoDependenciesFromSparseIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/se/rd/serde" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"name":"serde","vers":"1.0.1","deps":[{"name":"old","req":"^1","kind":"normal"},{"name":"opt","req":"^1","kind":"normal","optional":true}]}`+"\n")
		fmt.Fprint(w, `{"name":"serde","vers":"1.0.2","deps":[{"name":"new","req":"^1","kind":"normal"},{"name":"bench","req":"^1","kind":"dev"}]}`+"\n")
	}))
	t.Cleanup(srv.Close)

	c := clientFor(t, Cargo, srv.URL, "")
	// Asked for the version that is there, the line for that version answers - and
	// an optional dependency is a feature nobody asked for, and a dev one is the
	// test harness, so neither is on it.
	got := names(c.Dependencies(lang.Target{Ecosystem: Cargo, Package: "serde", Version: "1.0.1"}))
	if len(got) != 1 || got[0] != "old" {
		t.Errorf("got %v, want the dependency of the version asked for", got)
	}
	// Asked for nothing in particular, the newest published line does.
	got = names(c.Dependencies(lang.Target{Ecosystem: Cargo, Package: "serde"}))
	if len(got) != 1 || got[0] != "new" {
		t.Errorf("got %v, want the newest version's dependency", got)
	}
}

// Verifies: REQ-SUP-024
func TestSparsePath(t *testing.T) {
	for name, want := range map[string]string{
		"a":     "1/a",
		"go":    "2/go",
		"log":   "3/l/log",
		"serde": "se/rd/serde",
		"Serde": "se/rd/serde",
	} {
		if got := sparsePath(name); got != want {
			t.Errorf("sparsePath(%q) = %q, want %q", name, got, want)
		}
	}
}

// stubNuGet serves a service index, a version listing and a nuspec.
func stubNuGet(t *testing.T) *httptest.Server {
	t.Helper()
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/index.json":
			fmt.Fprintf(w, `{"resources":[
				{"@id":"%s/v3/registration/","@type":"RegistrationsBaseUrl/3.6.0"},
				{"@id":"%s/v3/flat2/","@type":"PackageBaseAddress/3.0.0"}]}`, base, base)
		case "/v3/flat2/serilog/index.json":
			fmt.Fprint(w, `{"versions":["3.0.0","3.1.1","4.0.0-dev"]}`)
		case "/v3/flat2/serilog/3.1.1/serilog.nuspec":
			fmt.Fprint(w, `<?xml version="1.0"?><package><metadata><id>Serilog</id>
				<dependencies>
					<dependency id="Flat.Dep" version="1.0.0" />
					<group targetFramework="net8.0"><dependency id="Grouped.Dep" version="2.0.0" /></group>
					<group targetFramework="net6.0"><dependency id="Grouped.Dep" version="2.0.0" /></group>
				</dependencies></metadata></package>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	base = srv.URL
	t.Cleanup(srv.Close)
	return srv
}

// Verifies: REQ-SUP-025
func TestNuGetDependencies(t *testing.T) {
	srv := stubNuGet(t)
	c := clientFor(t, NuGet, srv.URL+"/v3/index.json", "")
	got := names(c.Dependencies(lang.Target{Ecosystem: NuGet, Package: "Serilog", Version: "3.1.1"}))
	// Flat and grouped dependencies both count, and the same package named in two
	// framework groups is one dependency.
	if len(got) != 2 || got[0] != "Flat.Dep" || got[1] != "Grouped.Dep" {
		t.Errorf("got %v, want the flat and the grouped dependency once each", got)
	}
	// Without a version the feed's newest release answers - not its pre-release.
	if got = names(c.Dependencies(lang.Target{Ecosystem: NuGet, Package: "Serilog"})); len(got) != 2 {
		t.Errorf("got %v for the version the feed would install", got)
	}
}

// stubRegistry serves an OCI registry that demands a pull token first.
func stubRegistry(t *testing.T, manifest, blob string) (*httptest.Server, *[]string) {
	t.Helper()
	var asked []string
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if r.URL.Path == "/token" {
			fmt.Fprint(w, `{"token":"pull-token"}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer pull-token" {
			w.Header().Set("Www-Authenticate",
				fmt.Sprintf(`Bearer realm="%s/token",service="registry",scope="repository:library/app:pull"`, base))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v2/library/app/manifests/1.2":
			fmt.Fprint(w, manifest)
		case "/v2/library/app/blobs/sha256:cfg":
			fmt.Fprint(w, blob)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	base = srv.URL
	// An image reference carries its own registry, so a configured source cannot
	// point the client at the stub: the default one has to, for the length of the test.
	public[OCI] = srv.URL
	t.Cleanup(func() { public[OCI] = "https://registry-1.docker.io" })
	t.Cleanup(srv.Close)
	return srv, &asked
}

// Verifies: REQ-SUP-026, REQ-SUP-027
func TestOCIBaseFromConfigLabel(t *testing.T) {
	srv, asked := stubRegistry(t,
		`{"config":{"digest":"sha256:cfg"}}`,
		`{"config":{"Labels":{"org.opencontainers.image.base.name":"docker.io/library/debian:12-slim"}}}`)
	c := clientFor(t, OCI, srv.URL, "")
	deps := c.Dependencies(lang.Target{Ecosystem: OCI, Package: "app", Version: "1.2"})
	if len(deps) != 1 || deps[0].Package != "docker.io/library/debian" || deps[0].Version != "12-slim" {
		t.Fatalf("got %+v, want the base image the config labels name", deps)
	}
	// The token challenge was answered rather than given up on.
	for _, want := range []string{"/token", "/v2/library/app/blobs/sha256:cfg"} {
		found := false
		for _, path := range *asked {
			found = found || path == want
		}
		if !found {
			t.Errorf("never asked for %s; asked %v", want, *asked)
		}
	}
}

// Verifies: REQ-SUP-026
func TestOCIBaseFromManifestAnnotation(t *testing.T) {
	// An annotation on the manifest says it outright, and the digest beside it is
	// what was actually built on, so the tag is not what travels.
	srv, asked := stubRegistry(t, `{"config":{"digest":"sha256:cfg"},"annotations":{
		"org.opencontainers.image.base.name":"alpine:3.19",
		"org.opencontainers.image.base.digest":"sha256:abc"}}`, `{}`)
	c := clientFor(t, OCI, srv.URL, "")
	deps := c.Dependencies(lang.Target{Ecosystem: OCI, Package: "app", Version: "1.2"})
	if len(deps) != 1 || deps[0].Package != "alpine" || deps[0].Version != "sha256:abc" {
		t.Fatalf("got %+v, want the annotated base image at its digest", deps)
	}
	for _, path := range *asked {
		if path == "/v2/library/app/blobs/sha256:cfg" {
			t.Error("fetched the config blob although the manifest had already said")
		}
	}
}

// An image named without a tag carries no version (it floats); the registry is
// asked for the tag such a reference pulls, latest.
//
// Verifies: REQ-CI-013
func TestOCIUntaggedImageAsksForLatest(t *testing.T) {
	srv, asked := stubRegistry(t, `{}`, `{}`)
	c := clientFor(t, OCI, srv.URL, "")
	c.Dependencies(lang.Target{Ecosystem: OCI, Package: "app", Floating: true})
	found := false
	for _, path := range *asked {
		found = found || path == "/v2/library/app/manifests/latest"
	}
	if !found {
		t.Errorf("never asked for the latest manifest; asked %v", *asked)
	}
}

func TestOCIRepository(t *testing.T) {
	for image, want := range map[string]string{
		"alpine":             "library/alpine",
		"bitnami/nginx":      "bitnami/nginx",
		"ghcr.io/org/app":    "org/app",
		"localhost:5000/app": "app",
	} {
		if got := ociRepository(image); got != want {
			t.Errorf("ociRepository(%q) = %q, want %q", image, got, want)
		}
	}
}

func TestSplitChallenge(t *testing.T) {
	// A scope may hold commas of its own, and they are not parameter separators.
	got := splitChallenge(`realm="https://auth/token",service="reg",scope="repository:a/b:pull,push"`)
	if len(got) != 3 || got[2] != `scope="repository:a/b:pull,push"` {
		t.Errorf("got %q, want the scope kept whole", got)
	}
}

// Verifies: REQ-SUP-028
func TestMavenIsNotAsked(t *testing.T) {
	// A Maven package on the map is a group, and a POM needs a group and an artifact:
	// there is nothing to request, and nothing should be requested.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("asked the Maven repository for %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	c := clientFor(t, Maven, srv.URL, "")
	if deps := c.Dependencies(lang.Target{Ecosystem: Maven, Package: "org.slf4j", Version: "2.0.9"}); len(deps) != 0 {
		t.Errorf("got %v", names(deps))
	}
}

// A 401 with no challenge this client can answer is reported as a 401, not as the
// JSON parse error an empty body would make of it.
//
// Verifies: REQ-SUP-027, REQ-TRC-007
func TestOCIUnansweredChallengeSaysUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Www-Authenticate", `Basic realm="registry"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	c := clientFor(t, OCI, srv.URL, "")
	_, err := c.ociGet(context.Background(), srv.URL, "library/app", srv.URL+"/v2/library/app/manifests/1", "application/json")
	if err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("got %v, want an Unauthorized error", err)
	}
}

// stubComposer serves a Composer repository: packages.json naming where each
// package's metadata is, and minified Composer 2 metadata for one package.
func stubComposer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		switch r.URL.Path {
		case "/composer/packages.json":
			w.Write([]byte(`{"packages":[],"metadata-url":"/composer/p2/%package%.json"}`))
		case "/composer/p2/monolog/monolog.json":
			// Newest first; each entry only says what changed from the one above.
			fmt.Fprint(w, `{"minified":"composer/2.0","packages":{"monolog/monolog":[
				{"name":"monolog/monolog","version":"3.6.0","require":{"php":">=8.1","psr/log":"^2.0 || ^3.0"}},
				{"version":"3.5.0"},
				{"version":"2.9.1","require":{"php":">=7.2","psr/log":"^1.0.1 || ^2.0 || ^3.0","ext-json":"*"}},
				{"version":"1.0.0","require":"__unset"}
			]}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &asked
}

// Verifies: REQ-SUP-044
func TestComposerDependencies(t *testing.T) {
	srv, asked := stubComposer(t)
	c := clientFor(t, Composer, srv.URL+"/composer", "")
	for _, tt := range []struct {
		version string
		want    []lang.Target
	}{
		// An inherited require, the platform (php, ext-json) left out.
		{"3.5.0", []lang.Target{{Ecosystem: Composer, Package: "psr/log", Version: "^2.0 || ^3.0"}}},
		{"v2.9.1", []lang.Target{{Ecosystem: Composer, Package: "psr/log", Version: "^1.0.1 || ^2.0 || ^3.0"}}},
		// A range names no version: the newest answers.
		{"^3.0", []lang.Target{{Ecosystem: Composer, Package: "psr/log", Version: "^2.0 || ^3.0"}}},
		{"1.0.0", nil}, // "__unset": nothing required
	} {
		got := c.Dependencies(lang.Target{Ecosystem: Composer, Package: "monolog/monolog", Version: tt.version})
		if len(got) == 0 && len(tt.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %+v, want %+v", tt.version, got, tt.want)
		}
	}
	// packages.json is asked once for the repository, not once per package.
	n := 0
	for _, p := range *asked {
		if p == "/composer/packages.json" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("packages.json asked %d times: %v", n, *asked)
	}
}

// Verifies: REQ-SUP-044
func TestPackagistIsAskedDirectly(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if r.URL.Path != "/p2/log/log.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"packages":{"log/log":[{"version":"3.0.1","require":{"php":">=8.0.0"}}]}}`)
	}))
	defer srv.Close()
	public[Composer] = srv.URL
	t.Cleanup(func() { public[Composer] = "https://repo.packagist.org" })
	c := NewClient(New(), t.TempDir(), time.Hour, 5*time.Second, auth.Read("", nil), nil)
	got := c.Dependencies(lang.Target{Ecosystem: Composer, Package: "Log/Log", Version: "3.0.1"})
	if len(got) != 0 || !reflect.DeepEqual(asked, []string{"/p2/log/log.json"}) {
		t.Errorf("asked %v, got %+v; want one request for the lower-case name and no packages", asked, got)
	}
}

// Verifies: REQ-SUP-045
func TestRubyGemsDependencies(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if r.URL.Path != "/private/info/sinatra" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte("---\n" +
			"3.0.0 rack:~> 2.2&>= 2.2.4,tilt:~> 2.0|checksum:aa,ruby:>= 2.6.0\n" +
			"4.0.0 mustermann:~> 3.0,rack:< 4&>= 3.0,rack-protection:= 4.0.0|checksum:bb\n" +
			"4.0.0-java rack:< 4|checksum:cc\n" +
			"4.1.0.beta1 rack:>= 3.1|checksum:dd\n"))
	}))
	defer srv.Close()
	c := clientFor(t, RubyGems, srv.URL+"/private/", "")
	for _, tt := range []struct {
		version string
		want    []lang.Target
	}{
		{"3.0.0", []lang.Target{
			{Ecosystem: RubyGems, Package: "rack", Version: "~> 2.2, >= 2.2.4"},
			{Ecosystem: RubyGems, Package: "tilt", Version: "~> 2.0"},
		}},
		// No version: the newest release, not the pre-release nor a platform's build;
		// "= 4.0.0" is one version, and pinned.
		{"~> 4.0", []lang.Target{
			{Ecosystem: RubyGems, Package: "mustermann", Version: "~> 3.0"},
			{Ecosystem: RubyGems, Package: "rack", Version: "< 4, >= 3.0"},
			{Ecosystem: RubyGems, Package: "rack-protection", Version: "4.0.0", Pinned: true},
		}},
	} {
		got := c.Dependencies(lang.Target{Ecosystem: RubyGems, Package: "sinatra", Version: tt.version})
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %+v, want %+v", tt.version, got, tt.want)
		}
	}
	if len(asked) == 0 || asked[0] != "/private/info/sinatra" {
		t.Errorf("asked %v, want the compact index", asked)
	}
}

// Verifies: REQ-SUP-046
func TestPubDependencies(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path+" "+r.Header.Get("Accept"))
		if r.URL.Path != "/private/api/packages/http" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"name":"http",
"latest":{"version":"1.2.0","pubspec":{"name":"http","dependencies":{"async":"^2.5.0","meta":null,"web":">=0.5.0 <2.0.0","flutter":{"sdk":"flutter"}},"dev_dependencies":{"test":"^1.21.0"}}},
"versions":[
 {"version":"1.1.0","pubspec":{"name":"http","dependencies":{"async":"^2.5.0","http_parser":{"hosted":"https://pub.dev","version":"4.0.2"}}}},
 {"version":"1.2.0","pubspec":{"name":"http","dependencies":{"async":"^2.5.0","meta":null,"web":">=0.5.0 <2.0.0","flutter":{"sdk":"flutter"}}}}]}`))
	}))
	defer srv.Close()
	c := clientFor(t, Pub, srv.URL+"/private/", "")
	for _, tt := range []struct {
		version string
		want    []lang.Target
	}{
		// The version asked for; a bare version is pinned.
		{"1.1.0", []lang.Target{
			{Ecosystem: Pub, Package: "async", Version: "^2.5.0"},
			{Ecosystem: Pub, Package: "http_parser", Version: "4.0.2", Pinned: true},
		}},
		// A constraint names no version: the latest answers, without its SDK and
		// dev dependencies.
		{"^1.0.0", []lang.Target{
			{Ecosystem: Pub, Package: "async", Version: "^2.5.0"},
			{Ecosystem: Pub, Package: "meta"},
			{Ecosystem: Pub, Package: "web", Version: ">=0.5.0 <2.0.0"},
		}},
	} {
		got := c.Dependencies(lang.Target{Ecosystem: Pub, Package: "http", Version: tt.version})
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %+v, want %+v", tt.version, got, tt.want)
		}
	}
	if len(asked) == 0 || asked[0] != "/private/api/packages/http application/vnd.pub.v2+json" {
		t.Errorf("asked %v, want the package API", asked)
	}
}

// Verifies: REQ-SUP-047
func TestHexDependencies(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path+" "+r.Header.Get("Accept"))
		switch r.URL.Path {
		case "/api/packages/plug":
			w.Write([]byte(`{"name":"plug","latest_version":"1.17.0-rc.0","latest_stable_version":"1.16.1",
"releases":[{"version":"1.17.0-rc.0"},{"version":"1.16.1"},{"version":"1.15.0"}]}`))
		case "/api/packages/plug/releases/1.15.0":
			w.Write([]byte(`{"version":"1.15.0","requirements":{
"mime":{"app":"mime","optional":false,"requirement":"~> 1.0 or ~> 2.0"},
"plug_crypto":{"app":"plug_crypto","optional":false,"requirement":"== 2.0.0"}}}`))
		case "/api/packages/plug/releases/1.16.1":
			w.Write([]byte(`{"version":"1.16.1","requirements":{
"mime":{"app":"mime","optional":false,"requirement":"~> 2.0"},
"telemetry":{"app":"telemetry","optional":false,"requirement":"~> 0.4.3 or ~> 1.0"},
"jason":{"app":"jason","optional":true,"requirement":"~> 1.0"}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := clientFor(t, Hex, srv.URL+"/api", "")
	for _, tt := range []struct {
		version string
		want    []lang.Target
	}{
		// The release asked for; an exact requirement is pinned.
		{"1.15.0", []lang.Target{
			{Ecosystem: Hex, Package: "mime", Version: "~> 1.0 or ~> 2.0"},
			{Ecosystem: Hex, Package: "plug_crypto", Version: "2.0.0", Pinned: true},
		}},
		// A requirement names no release: the latest stable one answers, without
		// its optional requirements.
		{"~> 1.14", []lang.Target{
			{Ecosystem: Hex, Package: "mime", Version: "~> 2.0"},
			{Ecosystem: Hex, Package: "telemetry", Version: "~> 0.4.3 or ~> 1.0"},
		}},
	} {
		got := c.Dependencies(lang.Target{Ecosystem: Hex, Package: "plug", Version: tt.version})
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %+v, want %+v", tt.version, got, tt.want)
		}
	}
	if len(asked) == 0 || asked[0] != "/api/packages/plug application/json" {
		t.Errorf("asked %v, want the package first", asked)
	}
}

// CRAN and its mirrors are asked through crandb: the release asked for, else the
// current one; R and its base packages are not dependencies.
//
// Verifies: REQ-SUP-048
func TestCRANDependencies(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		switch r.URL.Path {
		case "/dplyr/1.1.4":
			w.Write([]byte(`{"Package":"dplyr","Version":"1.1.4",
"Depends":{"R":">= 3.5.0"},
"Imports":{"cli":">= 3.4.0","generics":"*","methods":"*","R6":"*","vctrs":">= 0.6.4"}}`))
		case "/dplyr":
			w.Write([]byte(`{"Package":"dplyr","Version":"1.1.5","Imports":{"cli":">= 3.6.0"},"LinkingTo":{"cpp11":"== 0.4.7"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	old := crandbAPI
	crandbAPI = srv.URL
	t.Cleanup(func() { crandbAPI = old })
	c := clientFor(t, CRAN, "https://cloud.r-project.org", "")
	for _, tt := range []struct {
		version string
		want    []lang.Target
	}{
		{"1.1.4", []lang.Target{
			{Ecosystem: CRAN, Package: "R6"},
			{Ecosystem: CRAN, Package: "cli", Version: ">= 3.4.0"},
			{Ecosystem: CRAN, Package: "generics"},
			{Ecosystem: CRAN, Package: "vctrs", Version: ">= 0.6.4"},
		}},
		// A requirement names no release; crandb does not know 1.0.99 either.
		{">= 1.1.0", []lang.Target{
			{Ecosystem: CRAN, Package: "cli", Version: ">= 3.6.0"},
			{Ecosystem: CRAN, Package: "cpp11", Version: "0.4.7", Pinned: true},
		}},
		{"1.0.99", []lang.Target{
			{Ecosystem: CRAN, Package: "cli", Version: ">= 3.6.0"},
			{Ecosystem: CRAN, Package: "cpp11", Version: "0.4.7", Pinned: true},
		}},
	} {
		got := c.Dependencies(lang.Target{Ecosystem: CRAN, Package: "dplyr", Version: tt.version})
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %+v, want %+v", tt.version, got, tt.want)
		}
	}
	if want := []string{"/dplyr/1.1.4", "/dplyr", "/dplyr/1.0.99", "/dplyr"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %v, want %v", asked, want)
	}
}

// Any other R repository is asked for its src/contrib/PACKAGES, once for all of its
// packages.
//
// Verifies: REQ-SUP-048
func TestCRANLikeRepositoryDependencies(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if r.URL.Path != "/drat/src/contrib/PACKAGES" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte("Package: acmeR\nVersion: 0.3.0\nDepends: R (>= 4.0), methods\nImports: dplyr (>= 1.1.0),\n    jsonlite\nLinkingTo: Rcpp\n\n" +
			"Package: acmeUtils\nVersion: 1.0.0\n"))
	}))
	defer srv.Close()
	c := clientFor(t, CRAN, srv.URL+"/drat", "")
	got := c.Dependencies(lang.Target{Ecosystem: CRAN, Package: "acmeR", Version: "0.3.0"})
	want := []lang.Target{
		{Ecosystem: CRAN, Package: "Rcpp"},
		{Ecosystem: CRAN, Package: "dplyr", Version: ">= 1.1.0"},
		{Ecosystem: CRAN, Package: "jsonlite"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := c.Dependencies(lang.Target{Ecosystem: CRAN, Package: "acmeUtils", Version: "1.0.0"}); len(got) != 0 {
		t.Errorf("acmeUtils: got %+v, want none", got)
	}
	if want := []string{"/drat/src/contrib/PACKAGES"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %v, want %v", asked, want)
	}
}

// A Hackage package is asked for its preferred versions, then for the package
// description of the version asked for (else the newest normal one): the
// build-depends of its libraries and the common stanzas they import, without GHC's own
// packages and the package's sublibraries; ==x pins.
//
// Verifies: REQ-SUP-049
func TestHackageDependencies(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		switch r.URL.Path {
		case "/package/acme-json/preferred":
			if r.Header.Get("Accept") != "application/json" {
				w.WriteHeader(http.StatusNotAcceptable)
				return
			}
			w.Write([]byte(`{"normal-version":["1.10.0","1.9.2","1.2.0"],"deprecated-version":["1.11.0"]}`))
		case "/package/acme-json-1.2.0/acme-json.cabal":
			w.Write([]byte("cabal-version: 3.0\nname: acme-json\nversion: 1.2.0\n\n" +
				"common deps\n  build-depends: containers ^>=0.6\n\n" +
				"library\n  import: deps\n  build-depends:\n      base >=4.14 && <5\n    , text ==2.0.2\n" +
				"    , acme-json:internal\n  if flag(fast)\n    build-depends: vector\n\n" +
				"library internal\n  build-depends: bytestring, template-haskell\n\n" +
				"test-suite spec\n  build-depends: hspec\n"))
		case "/package/acme-json-1.10.0/acme-json.cabal":
			w.Write([]byte("name: acme-json\nlibrary\n  build-depends: base, aeson:{aeson, attoparsec-aeson} >=2.2\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := clientFor(t, Hackage, srv.URL, "")
	got := c.Dependencies(lang.Target{Ecosystem: Hackage, Package: "acme-json", Version: "1.2.0", Pinned: true})
	want := []lang.Target{
		{Ecosystem: Hackage, Package: "bytestring"},
		{Ecosystem: Hackage, Package: "containers", Version: "^>=0.6"},
		{Ecosystem: Hackage, Package: "text", Version: "2.0.2", Pinned: true},
		{Ecosystem: Hackage, Package: "vector"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("1.2.0: got %+v, want %+v", got, want)
	}
	// A range names no release: the newest normal version (1.10.0, not 1.9.2 and not
	// the deprecated 1.11.0) answers.
	got = c.Dependencies(lang.Target{Ecosystem: Hackage, Package: "acme-json", Version: ">=1.2"})
	if want := []lang.Target{{Ecosystem: Hackage, Package: "aeson", Version: ">=2.2"}}; !reflect.DeepEqual(got, want) {
		t.Errorf(">=1.2: got %+v, want %+v", got, want)
	}
	if want := []string{"/package/acme-json/preferred", "/package/acme-json-1.2.0/acme-json.cabal",
		"/package/acme-json/preferred", "/package/acme-json-1.10.0/acme-json.cabal"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %v, want %v", asked, want)
	}
}

// A Terraform registry module is asked for its versions (the modules.v1 path read
// from the registry's service discovery document): the version asked for, else the
// newest release its constraint allows, answers with the providers and registry
// modules its root module - or the submodule named after // - requires. Providers
// are their own island; git and local module sources are left out.
//
// Verifies: REQ-SUP-050
func TestTerraformModuleDependencies(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		switch r.URL.Path {
		case "/.well-known/terraform.json":
			w.Write([]byte(`{"modules.v1":"/api/registry/v1/modules/","providers.v1":"/api/registry/v1/providers/"}`))
		case "/api/registry/v1/modules/acme/vpc/aws/versions":
			w.Write([]byte(`{"modules":[{"source":"acme/vpc/aws","versions":[
				{"version":"5.1.0","root":{"providers":[{"name":"aws","namespace":"hashicorp","source":"hashicorp/aws","version":">= 5.0"}],"dependencies":[]}},
				{"version":"5.2.0","root":{"providers":[{"name":"aws","namespace":"hashicorp","source":"registry.terraform.io/hashicorp/aws","version":">= 5.20"},{"name":"random","version":""}],
					"dependencies":[{"name":"labels","source":"cloudposse/label/null","version":"= 0.25.0"},{"name":"local","source":"./modules/x","version":""},{"name":"git","source":"git::https://example.com/m.git","version":""}]},
				 "submodules":[{"path":"modules/endpoints","providers":[{"name":"aws","namespace":"hashicorp","source":"hashicorp/aws","version":">= 5.1"}],"dependencies":[]}]},
				{"version":"6.0.0","root":{"providers":[{"name":"aws","source":"hashicorp/aws","version":">= 6.0"}]}},
				{"version":"5.3.0-beta1","root":{"providers":[]}}]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := clientFor(t, TerraformModule, srv.URL, "")
	got := c.Dependencies(lang.Target{Ecosystem: TerraformModule, Package: "acme/vpc/aws", Version: "~> 5.1"})
	want := []lang.Target{
		{Ecosystem: TerraformModule, Package: "cloudposse/label/null", Version: "0.25.0", Pinned: true},
		{Ecosystem: "terraform-provider", Package: "hashicorp/aws", Version: ">= 5.20"},
		{Ecosystem: "terraform-provider", Package: "hashicorp/random"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("~> 5.1: got %+v, want %+v", got, want)
	}
	got = c.Dependencies(lang.Target{Ecosystem: TerraformModule, Package: "acme/vpc/aws//modules/endpoints", Version: "5.2.0", Pinned: true})
	if want := []lang.Target{{Ecosystem: "terraform-provider", Package: "hashicorp/aws", Version: ">= 5.1"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("submodule: got %+v, want %+v", got, want)
	}
	got = c.Dependencies(lang.Target{Ecosystem: TerraformModule, Package: "acme/vpc/aws", Floating: true})
	if want := []lang.Target{{Ecosystem: "terraform-provider", Package: "hashicorp/aws", Version: ">= 6.0"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("no version: got %+v, want %+v", got, want)
	}
	// Service discovery is asked once per registry.
	if want := []string{"/.well-known/terraform.json", "/api/registry/v1/modules/acme/vpc/aws/versions",
		"/api/registry/v1/modules/acme/vpc/aws/versions", "/api/registry/v1/modules/acme/vpc/aws/versions"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %v, want %v", asked, want)
	}
}

// Verifies: REQ-SUP-050
func TestTerraformConstraints(t *testing.T) {
	for _, tc := range []struct {
		constraint, version string
		want                bool
	}{
		{"", "1.0.0", true}, {"1.2.3", "1.2.3", true}, {"= 1.2.3", "1.2.4", false},
		{"~> 1.2", "1.9.0", true}, {"~> 1.2", "2.0.0", false}, {"~> 1.2.0", "1.2.9", true}, {"~> 1.2.0", "1.3.0", false},
		{">= 4.0, < 6.0", "5.10.0", true}, {">= 4.0, < 6.0", "6.0.0", false}, {"!= 1.0.0", "1.0.0", false},
	} {
		if got := terraformAllows(tc.constraint, tc.version); got != tc.want {
			t.Errorf("%q allows %s: got %v", tc.constraint, tc.version, got)
		}
	}
}

// A pod is looked up on the CocoaPods CDN by the MD5 shard of its name: the version
// asked for, else the newest release that is not a pre-release and that the
// requirement allows (from the shard's version list), answers with its podspec's dependencies - the root spec's and its
// default subspec's, not the pod's own subspecs or the other subspecs'; "= x" pins.
//
// Verifies: REQ-SUP-051
func TestCocoaPodsDependencies(t *testing.T) {
	sum := md5.Sum([]byte("AcmeKit"))
	h := hex.EncodeToString(sum[:])
	shard := h[0:1] + "/" + h[1:2] + "/" + h[2:3]
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		switch r.URL.Path {
		case "/all_pods_versions_" + strings.ReplaceAll(shard, "/", "_") + ".txt":
			w.Write([]byte("AcmeKitty/9.0.0\nAcmeKit/1.2.0/1.9.0/1.10.0/2.0.0-beta.1/2.1.0\n"))
		case "/Specs/" + shard + "/AcmeKit/1.2.0/AcmeKit.podspec.json":
			w.Write([]byte(`{"name":"AcmeKit","version":"1.2.0","dependencies":{"AFNetworking":["~> 4.0"]},
				"default_subspecs":"Core","subspecs":[
				{"name":"Core","dependencies":{"AcmeKit/Base":[],"Mantle/extobjc":["= 2.2.0"]}},
				{"name":"Extras","dependencies":{"PromiseKit":[]}}]}`))
		case "/Specs/" + shard + "/AcmeKit/1.10.0/AcmeKit.podspec.json":
			w.Write([]byte(`{"name":"AcmeKit","version":"1.10.0","subspecs":[{"name":"A","dependencies":{"SDWebImage/Core":[">= 5.0", "< 6.0"]}}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := clientFor(t, CocoaPods, srv.URL, "")
	got := c.Dependencies(lang.Target{Ecosystem: CocoaPods, Package: "AcmeKit", Version: "1.2.0", Pinned: true})
	want := []lang.Target{
		{Ecosystem: CocoaPods, Package: "AFNetworking", Version: "~> 4.0"},
		{Ecosystem: CocoaPods, Package: "Mantle", Version: "2.2.0", Pinned: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("1.2.0: got %+v, want %+v", got, want)
	}
	got = c.Dependencies(lang.Target{Ecosystem: CocoaPods, Package: "AcmeKit", Version: "~> 1.0", Floating: true})
	if want := []lang.Target{{Ecosystem: CocoaPods, Package: "SDWebImage", Version: ">= 5.0, < 6.0"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("~> 1.0: got %+v, want %+v", got, want)
	}
	if want := []string{"/Specs/" + shard + "/AcmeKit/1.2.0/AcmeKit.podspec.json",
		"/all_pods_versions_" + strings.ReplaceAll(shard, "/", "_") + ".txt",
		"/Specs/" + shard + "/AcmeKit/1.10.0/AcmeKit.podspec.json"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %v, want %v", asked, want)
	}
}
