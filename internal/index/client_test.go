package index

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// stubIndex serves what each ecosystem's index serves, so the client can be checked
// without the network.
func stubIndex(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var asked []string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if r.Header.Get("Authorization") == "" && r.URL.Path == "/private/1.0.0" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/example.com/mod/@v/v1.2.3.mod":
			fmt.Fprint(w, "module example.com/mod\n\ngo 1.22\n\nrequire (\n\tgithub.com/direct/dep v1.0.0\n\tgithub.com/indirect/dep v1.5.0 // indirect\n)\n")
		case "/react/18.3.1":
			fmt.Fprint(w, `{"name":"react","dependencies":{"loose-envify":"^1.1.0"}}`)
		case "/react/latest":
			fmt.Fprint(w, `{"name":"react","dependencies":{"loose-envify":"^1.1.0","js-tokens":"^4"}}`)
		case "/pypi/requests/2.31.0/json":
			fmt.Fprint(w, `{"info":{"requires_dist":["certifi (>=2017.4.17)","urllib3 (<3,>=1.21.1)","PySocks ; extra == 'socks'"]}}`)
		case "/private/1.0.0":
			fmt.Fprint(w, `{"name":"private","dependencies":{"secret-dep":"1.0.0"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &asked
}

func clientFor(t *testing.T, eco, url, home string) *Client {
	t.Helper()
	cfg := New()
	cfg.Add(eco, Source{URL: url, Trusted: true})
	return NewClient(cfg, t.TempDir(), time.Hour, 5*time.Second, home)
}

func names(deps []lang.Target) []string {
	var out []string
	for _, d := range deps {
		out = append(out, d.Package)
	}
	sort.Strings(out)
	return out
}

func TestGoModuleDependencies(t *testing.T) {
	srv, _ := stubIndex(t)
	c := clientFor(t, Go, srv.URL, "")
	got := names(c.Dependencies(lang.Target{Ecosystem: Go, Package: "example.com/mod", Version: "v1.2.3"}))
	// An indirect requirement belongs to something else's go.mod, not to this module.
	if len(got) != 1 || got[0] != "github.com/direct/dep" {
		t.Errorf("got %v, want the direct requirement only", got)
	}
	// Without a version the proxy has no document to serve, and nothing is asked.
	if deps := c.Dependencies(lang.Target{Ecosystem: Go, Package: "example.com/mod"}); len(deps) != 0 {
		t.Errorf("got %v for a module with no version", names(deps))
	}
}

func TestNpmDependencies(t *testing.T) {
	srv, asked := stubIndex(t)
	c := clientFor(t, NPM, srv.URL, "")
	if got := names(c.Dependencies(lang.Target{Ecosystem: NPM, Package: "react", Version: "18.3.1"})); len(got) != 1 || got[0] != "loose-envify" {
		t.Errorf("got %v", got)
	}
	// A range names no document, so the registry is asked for the current version.
	if got := names(c.Dependencies(lang.Target{Ecosystem: NPM, Package: "react", Version: "^18.0.0"})); len(got) != 2 {
		t.Errorf("got %v, want what latest requires", got)
	}
	if len(*asked) != 2 || (*asked)[1] != "/react/latest" {
		t.Errorf("asked %v", *asked)
	}
}

func TestPyPIDependencies(t *testing.T) {
	srv, _ := stubIndex(t)
	cfg := New()
	cfg.Add(PyPI, Source{URL: srv.URL + "/simple", Trusted: true})
	c := NewClient(cfg, t.TempDir(), time.Hour, 5*time.Second, "")
	got := names(c.Dependencies(lang.Target{Ecosystem: PyPI, Package: "requests", Version: "2.31.0"}))
	// An extra's dependency is installed only when that extra is asked for.
	if len(got) != 2 || got[0] != "certifi" || got[1] != "urllib3" {
		t.Errorf("got %v, want certifi and urllib3", got)
	}
}

func TestAnIndexOnlyTheRepositoryNamesIsNotAsked(t *testing.T) {
	srv, asked := stubIndex(t)
	cfg := New()
	cfg.Add(NPM, Source{URL: srv.URL}) // as a repository's .npmrc would
	c := NewClient(cfg, t.TempDir(), time.Hour, 5*time.Second, "")
	if deps := c.Dependencies(lang.Target{Ecosystem: NPM, Package: "react", Version: "18.3.1"}); len(deps) != 0 {
		t.Errorf("got %v from an index nothing here vouches for", names(deps))
	}
	if len(*asked) != 0 {
		t.Errorf("the index was asked anyway: %v", *asked)
	}
}

func TestCredentialsGoToTheHostTheyWereWrittenFor(t *testing.T) {
	srv, _ := stubIndex(t)
	home := t.TempDir()
	host := srv.Listener.Addr().String()
	npmrc := fmt.Sprintf("//%s/:_authToken=secret-token\n", host)
	if err := os.WriteFile(filepath.Join(home, ".npmrc"), []byte(npmrc), 0o600); err != nil {
		t.Fatal(err)
	}
	c := clientFor(t, NPM, srv.URL, home)
	if got := names(c.Dependencies(lang.Target{Ecosystem: NPM, Package: "private", Version: "1.0.0"})); len(got) != 1 {
		t.Errorf("got %v, want the private package's dependency", got)
	}
	// The same request without the token is refused by the stub, which is what
	// proves the header was sent.
	plain := clientFor(t, NPM, srv.URL, t.TempDir())
	if got := plain.Dependencies(lang.Target{Ecosystem: NPM, Package: "private", Version: "1.0.0"}); len(got) != 0 {
		t.Errorf("got %v without credentials", names(got))
	}
}

func TestAnswersAreCached(t *testing.T) {
	srv, asked := stubIndex(t)
	dir := t.TempDir()
	cfg := New()
	cfg.Add(Go, Source{URL: srv.URL, Trusted: true})
	target := lang.Target{Ecosystem: Go, Package: "example.com/mod", Version: "v1.2.3"}

	first := NewClient(cfg, dir, time.Hour, 5*time.Second, "")
	first.Dependencies(target)
	// A second client, a second run: the answer is on disk.
	second := NewClient(cfg, dir, time.Hour, 5*time.Second, "")
	if got := names(second.Dependencies(target)); len(got) != 1 {
		t.Errorf("got %v from the cache", got)
	}
	if len(*asked) != 1 {
		t.Errorf("the index was asked %d times", len(*asked))
	}
	// An expired answer is asked for again.
	third := NewClient(cfg, dir, time.Nanosecond, 5*time.Second, "")
	third.Dependencies(target)
	if len(*asked) != 2 {
		t.Errorf("a stale answer was reused: %v", *asked)
	}
}

func TestNetrcCredentials(t *testing.T) {
	home := t.TempDir()
	netrc := "machine index.internal login user password pass\nmachine other.internal login u2 password p2\n"
	if err := os.WriteFile(filepath.Join(home, ".netrc"), []byte(netrc), 0o600); err != nil {
		t.Fatal(err)
	}
	c := readCredentials(home)
	req, _ := http.NewRequest(http.MethodGet, "https://index.internal/simple/requests/", nil)
	c.apply(req)
	if got := req.Header.Get("Authorization"); got != "Basic dXNlcjpwYXNz" {
		t.Errorf("got %q", got)
	}
	other, _ := http.NewRequest(http.MethodGet, "https://elsewhere.internal/x", nil)
	c.apply(other)
	if got := other.Header.Get("Authorization"); got != "" {
		t.Errorf("credentials were sent to another host: %q", got)
	}
}
