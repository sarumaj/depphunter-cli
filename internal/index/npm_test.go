package index

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// npmStub is an npm registry that answers every package document, but only to a
// request carrying want as its Authorization header.
type npmStub struct {
	*httptest.Server
	mu    sync.Mutex
	asked []string
}

func newNpmStub(t *testing.T, want string) *npmStub {
	t.Helper()
	s := &npmStub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.asked = append(s.asked, r.URL.Path+" "+r.Header.Get("Authorization"))
		s.mu.Unlock()
		if r.Header.Get("Authorization") != want {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"name":"x","dependencies":{"dep":"1.0.0"}}`)
	}))
	t.Cleanup(s.Close)
	return s
}

// authOf is the Authorization header the store sends to u.
func authOf(s *auth.Store, u string) string {
	request, _ := http.NewRequest(http.MethodGet, u, nil)
	s.Apply(request)
	return request.Header.Get("Authorization")
}

// sources lists an ecosystem's sources as "scope url" with "?" for untrusted.
func sources(c *Config, ecosystem string) []string {
	var out []string
	for _, s := range c.Sources(ecosystem) {
		mark := ""
		if !s.Trusted {
			mark = "?"
		}
		out = append(out, strings.TrimSpace(s.Scope+" "+s.URL+mark))
	}
	return out
}

// This machine's Yarn Berry (YARN_NPM_REGISTRY_SERVER over the file, ${VAR}
// resolved), Yarn 1 and Bun files name trusted registries; a repository's
// .yarnrc.yml, .yarnrc and bunfig.toml name untrusted ones, and a repository URL
// made of a variable is not recorded.
//
// Verifies: REQ-SUP-015, REQ-SUP-064
func TestYarnAndBunRegistries(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".yarnrc.yml"), "npmRegistryServer: https://file.corp\nnpmScopes:\n  acme:\n    npmRegistryServer: \"https://${ACME_HOST}/npm\"\n")
	put(t, filepath.Join(home, ".yarnrc"), "registry \"https://classic.corp\"\n\"@old:registry\" \"https://old.corp\"\n")
	put(t, filepath.Join(home, ".bunfig.toml"), "[install.scopes]\nbun = { url = \"https://u:p@bun.corp/\" }\n")
	c := discoverOn(home, "linux", map[string]string{"YARN_NPM_REGISTRY_SERVER": "https://env.corp", "ACME_HOST": "acme.corp"})
	c.project(write(t, map[string]string{
		"sub/.yarnrc.yml": "npmRegistryServer: \"${EVIL}\"\nnpmScopes:\n  repo:\n    npmRegistryServer: https://repo.corp\n",
		".yarnrc":         "\"@y1:registry\" \"https://y1.corp\"\n",
		"bunfig.toml":     "[install]\nregistry = \"https://:literal@bunrepo.corp/\"\n",
	}))
	got := strings.Join(sources(c, NPM), ", ")
	want := "https://env.corp, @acme https://acme.corp/npm, https://classic.corp, @old https://old.corp, @bun https://bun.corp, " +
		"@y1 https://y1.corp?, https://bunrepo.corp?, @repo https://repo.corp?"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// A package of a scope this machine's Yarn Berry configuration maps to a registry
// is asked there, with the scope's token.
//
// Verifies: REQ-AUTH-024, REQ-SUP-015
func TestYarnScopeRegistryAndToken(t *testing.T) {
	registry := newNpmStub(t, "Bearer acme-secret")
	home := t.TempDir()
	put(t, filepath.Join(home, ".yarnrc.yml"), "npmScopes:\n  acme:\n    npmRegistryServer: "+registry.URL+"/npm\n    npmAuthToken: ${ACME_TOKEN}\n")
	e := environment(map[string]string{"ACME_TOKEN": "acme-secret"})
	store := auth.Read(home, e)
	d := NewDiscoverer(e, home)
	d.Config().Credentials(store)
	config := d.Discover(nil)
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, store, nil)
	if got := names(c.Dependencies(lang.Target{Ecosystem: NPM, Package: "@acme/ui", Version: "1.0.0"})); len(got) != 1 {
		t.Errorf("got %v, asked %v", got, registry.asked)
	}
	if len(registry.asked) != 1 || !strings.HasPrefix(registry.asked[0], "/npm/@acme/ui/1.0.0 ") {
		t.Errorf("asked %v", registry.asked)
	}
}

// A repository's .yarnrc.yml and bunfig.toml may bind a registry to a variable of
// this machine's environment. The secret is lent only to a registry vouched for
// (--trust-index, or a registry of this machine's own on the host); a token
// written out, or one only a fallback fills, is discarded; a lent credential never
// replaces the machine's own.
//
// Verifies: REQ-AUTH-023
func TestRepositoryYarnAndBunCredentials(t *testing.T) {
	variables := map[string]string{"NPM_TOKEN": "env-token", "BUN_PASS": "env-pass", "IDENT": "ci:env-ident"}
	files := write(t, map[string]string{
		".yarnrc.yml": `npmRegistryServer: "https://yarn.corp/npm"
npmAuthToken: "${NPM_TOKEN}"
npmAlwaysAuth: true
npmScopes:
  lit:
    npmRegistryServer: "https://literal.corp"
    npmAuthToken: "written-out"
  fb:
    npmRegistryServer: "https://fallback.corp"
    npmAuthToken: "${UNSET_HERE:-repo-default}"
  held:
    npmRegistryServer: "https://held.corp"
    npmAuthToken: "${NPM_TOKEN}"
npmRegistries:
  "https://ident.corp":
    npmAuthIdent: "${IDENT}"
    npmAlwaysAuth: true
`,
		"bunfig.toml": `[install.scopes]
bun = { url = "https://bun.corp/", username = "ci", password = "$BUN_PASS" }
lit = { url = "https://bunlit.corp/", token = "written-out" }
url = "https://ci:written@bunurl.corp/"
`,
	})
	run := func(home string, trust ...string) *auth.Store {
		e := environment(variables)
		store := auth.Read(home, e)
		d := NewDiscoverer(e, home)
		d.Config().Credentials(store)
		d.Config().Trust(trust)
		d.Discover(files)
		return store
	}
	all := []string{"https://yarn.corp/npm/x", "https://ident.corp/x", "https://bun.corp/x",
		"https://literal.corp/x", "https://fallback.corp/x", "https://bunlit.corp/x", "https://bunurl.corp/x"}

	// Nobody vouches: nothing is lent.
	s := run(t.TempDir())
	for _, u := range all {
		if got := authOf(s, u); got != "" {
			t.Errorf("unvouched %s got %q", u, got)
		}
	}

	// Vouched with --trust-index: only the environment's secrets are lent.
	s = run(t.TempDir(), "https://yarn.corp/npm", "ident.corp", "bun.corp", "literal.corp", "fallback.corp", "bunlit.corp", "bunurl.corp")
	want := map[string]string{
		"https://yarn.corp/npm/x": "Bearer env-token",
		"https://ident.corp/x":    basicHeaderOf("ci:env-ident"),
		"https://bun.corp/x":      basicHeaderOf("ci:env-pass"),
	}
	for _, u := range all {
		if got := authOf(s, u); got != want[u] {
			t.Errorf("vouched %s: %q, want %q", u, got, want[u])
		}
	}
	// The lent token serves the registry's path, not the rest of its host.
	if got := authOf(s, "https://yarn.corp/wiki"); got != "" {
		t.Errorf("the lent token reached another path: %q", got)
	}

	// A registry this machine configures on the host vouches for it; the machine's
	// own credential there is kept.
	home := t.TempDir()
	put(t, filepath.Join(home, ".npmrc"), "@corp:registry=https://yarn.corp/npm\n//bun.corp/:_authToken=machine\n@b:registry=https://bun.corp/\n"+
		"@h:registry=https://held.corp/\n//held.corp/:_authToken=machine\n")
	s = run(home)
	if got := authOf(s, "https://yarn.corp/npm/x"); got != "Bearer env-token" {
		t.Errorf("machine host: %q", got)
	}
	if got := authOf(s, "https://bun.corp/x"); got != "Bearer machine" {
		t.Errorf("the machine's credential was replaced: %q", got)
	}
	if got := authOf(s, "https://held.corp/@held%2fx/1.0.0"); got != "Bearer machine" {
		t.Errorf("a scope's lent token took the machine's registry: %q", got)
	}
	if got := authOf(s, "https://ident.corp/x"); got != "" {
		t.Errorf("a host the machine does not name was lent to: %q", got)
	}
}

// basicHeaderOf is the Basic Authorization header of a pair.
func basicHeaderOf(pair string) string {
	request, _ := http.NewRequest(http.MethodGet, "https://x", nil)
	user, pass, _ := strings.Cut(pair, ":")
	request.SetBasicAuth(user, pass)
	return request.Header.Get("Authorization")
}

// Two npm registries on one host, each with a path-scoped token in the user's
// npmrc: each package is asked with its own registry's token.
//
// Verifies: REQ-AUTH-025
func TestPathScopedNpmTokensEndToEnd(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.URL.Path] = r.Header.Get("Authorization")
		mu.Unlock()
		want := map[string]string{"/p/1/": "Bearer one", "/p/2/": "Bearer two"}[r.URL.Path[:5]]
		if r.Header.Get("Authorization") != want {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"name":"x","dependencies":{"dep":"1.0.0"}}`)
	}))
	t.Cleanup(server.Close)
	host := server.Listener.Addr().String()
	home := t.TempDir()
	put(t, filepath.Join(home, ".npmrc"), fmt.Sprintf(
		"@one:registry=%[2]s/p/1/\n@two:registry=%[2]s/p/2/\n//%[1]s/p/1/:_authToken=one\n//%[1]s/p/2/:_authToken=two\n", host, server.URL))
	store := auth.Read(home, nil)
	d := NewDiscoverer(nil, home)
	d.Config().Credentials(store)
	c := NewClient(d.Discover(nil), t.TempDir(), time.Hour, 5*time.Second, store, nil)
	for _, packageName := range []string{"@one/a", "@two/b"} {
		if got := names(c.Dependencies(lang.Target{Ecosystem: NPM, Package: packageName, Version: "1.0.0"})); len(got) != 1 {
			t.Errorf("%s: got %v, seen %v", packageName, got, seen)
		}
	}
	if got := authOf(store, server.URL+"/p/3/x"); got != "" {
		t.Errorf("another path of the host got %q", got)
	}
}
