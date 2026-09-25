package index

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
