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

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// hexStub is a Hex API under /api: an organization's packages answer only to its
// key, as hex.pm's do, and the public side holds a package of the same name as the
// organization's - the one a dependency-confusion attack would publish.
type hexStub struct {
	*httptest.Server
	mu    sync.Mutex
	asked []string // "path Authorization"
}

func newHexStub(t *testing.T, key string) *hexStub {
	s := &hexStub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.asked = append(s.asked, r.URL.Path+" "+r.Header.Get("Authorization"))
		s.mu.Unlock()
		if strings.HasPrefix(r.URL.Path, "/api/repos/") && r.Header.Get("Authorization") != key {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.URL.Path {
		case "/api/repos/acme/packages/billing", "/api/packages/billing", "/api/packages/jason":
			fmt.Fprint(w, `{"latest_stable_version":"1.2.0","releases":[{"version":"1.2.0"}]}`)
		case "/api/repos/acme/packages/billing/releases/1.2.0":
			fmt.Fprint(w, `{"requirements":{"ledger":{"requirement":"~> 2.0","optional":false},"jason":{"requirement":"~> 1.4","optional":false}}}`)
		case "/api/packages/billing/releases/1.2.0":
			fmt.Fprint(w, `{"requirements":{"evil":{"requirement":"~> 1.0","optional":false}}}`)
		case "/api/packages/jason/releases/1.2.0":
			fmt.Fprint(w, `{"requirements":{}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *hexStub) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.asked)
}

// A package of a private Hex organization is asked of <api>/repos/<org>/ with this
// machine's key, and of nothing else: its dependencies are taken to be the
// organization's too (the API does not say otherwise), a package the organization
// lacks is not looked for among the public packages, and without a key nothing is
// asked and the report says why. A public package is asked as before, without the
// key; a repository other than hex.pm's organizations has no API to ask.
//
// Verifies: REQ-SUP-047, REQ-AUTH-028, REQ-BEAM-013
func TestHexOrganizationPackagesAskTheirOrganization(t *testing.T) {
	stub := newHexStub(t, "user-key")
	home := t.TempDir()
	variables := map[string]string{"HEX_API_URL": stub.URL + "/api", "HEX_API_KEY": "user-key"}
	config := Discover(nil, environment(variables), home)
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, environment(variables)), nil)
	org := stub.URL + "/api/repos/acme"

	billing := lang.Target{Ecosystem: Hex, Package: "billing", Version: "1.2.0", Registry: "hexpm:acme"}
	if index, known := config.ForTarget(billing); index != org || !known {
		t.Errorf("billing attributed to %s (known %v)", index, known)
	}
	dependencies := c.Dependencies(billing)
	if got := names(dependencies); !slices.Equal(got, []string{"jason", "ledger"}) {
		t.Fatalf("billing: %v", got)
	}
	for _, d := range dependencies {
		if d.Registry != "hexpm:acme" {
			t.Errorf("%s: registry %q", d.Package, d.Registry)
		}
	}
	if at, known, ok := c.Located(Hex, "billing"); !ok || !known || at != org {
		t.Errorf("billing located at %s (%v %v)", at, known, ok)
	}
	// Missing from the organization: not found, and not asked of the public side.
	if got, l := ask(t, c, lang.Target{Ecosystem: Hex, Package: "ledger", Version: "2.0.0", Registry: "hexpm:acme"}); len(got) != 0 || l.Index != org {
		t.Errorf("ledger: %v from %s (%s)", got, l.Index, l.Reason)
	}
	// A public package: hex.pm's side, without the key.
	if got := names(c.Dependencies(lang.Target{Ecosystem: Hex, Package: "jason", Version: "1.2.0"})); len(got) != 0 {
		t.Errorf("jason: %v", got)
	}
	// Another repository serves Hex's protobuf registry, not this API.
	for _, registrySpec := range []string{"mini_repo", "hexpm:../admin", "hexpm:"} {
		if _, l := ask(t, c, lang.Target{Ecosystem: Hex, Package: "mini", Version: "0.1.0", Registry: registrySpec}); l.Reason != trace.ReasonNoIndex {
			t.Errorf("%s: %q", registrySpec, l.Reason)
		}
	}
	for _, line := range stub.requests() {
		path, key, _ := strings.Cut(line, " ")
		switch {
		case strings.HasPrefix(path, "/api/packages/billing"), strings.HasPrefix(path, "/api/packages/ledger"):
			t.Errorf("an organization's package was named to the public side: %s", path)
		case strings.HasPrefix(path, "/api/repos/acme/") && key != "user-key":
			t.Errorf("%s sent %q", path, key)
		case strings.HasPrefix(path, "/api/packages/") && key != "":
			t.Errorf("the key went to a public package: %s", path)
		}
	}
	if !slices.Contains(stub.requests(), "/api/packages/jason ") {
		t.Errorf("jason was not asked: %v", stub.requests())
	}

	// Without a key: nothing asked, the reason reported.
	before := len(stub.requests())
	keyless := map[string]string{"HEX_API_URL": stub.URL + "/api"}
	c = NewClient(Discover(nil, environment(keyless), home), t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, environment(keyless)), nil)
	if got, l := ask(t, c, billing); len(got) != 0 || l.Reason != trace.ReasonNoKey || l.Index != org {
		t.Errorf("without a key: %v, %q at %s", got, l.Reason, l.Index)
	}
	if after := stub.requests()[before:]; len(after) != 0 {
		t.Errorf("asked without a key: %v", after)
	}
}

// The key of hex.config's hexpm:<org> repository reaches its organization end to
// end, found through HEX_HOME.
//
// Verifies: REQ-AUTH-028, REQ-SUP-047
func TestHexOrganizationKeyFromHexConfig(t *testing.T) {
	stub := newHexStub(t, "acme-key")
	home, hexHome := t.TempDir(), t.TempDir()
	body := fmt.Sprintf("{api_url,<<%q>>}.\n{'$repos',#{<<\"hexpm:acme\">> => #{auth_key => <<\"acme-key\">>}}}.\n", stub.URL+"/api")
	if err := os.WriteFile(filepath.Join(hexHome, "hex.config"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	variables := map[string]string{"HEX_HOME": hexHome}
	c := NewClient(Discover(nil, environment(variables), home), t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, environment(variables)), nil)
	if got := names(c.Dependencies(lang.Target{Ecosystem: Hex, Package: "billing", Version: "1.2.0", Registry: "hexpm:acme"})); !slices.Equal(got, []string{"jason", "ledger"}) {
		t.Errorf("billing: %v (asked %v)", got, stub.requests())
	}
}

// A pub-tokens.json token reaches the hosted repository it was added for, and only
// its path; an "env" entry takes the variable it names.
//
// Verifies: REQ-AUTH-027, REQ-SUP-046
func TestPubTokenEndToEnd(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.URL.Path+" "+r.Header.Get("Authorization"))
		mu.Unlock()
		if r.URL.Path != "/acme/api/packages/billing" || r.Header.Get("Authorization") != "Bearer pub-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"latest":{"version":"1.0.0","pubspec":{"dependencies":{"ledger":"^2.0.0"}}},"versions":[]}`)
	}))
	t.Cleanup(server.Close)
	home := t.TempDir()
	tokens := fmt.Sprintf(`{"version":1,"hosted":[{"url":%q,"env":"PUB_ACME_TOKEN"}]}`, server.URL+"/acme")
	directory := filepath.Join(home, ".config", "dart")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "pub-tokens.json"), []byte(tokens), 0o600); err != nil {
		t.Fatal(err)
	}
	variables := map[string]string{"PUB_HOSTED_URL": server.URL + "/acme", "PUB_ACME_TOKEN": "pub-secret"}
	store := auth.Read(home, environment(variables))
	c := NewClient(Discover(nil, environment(variables), home), t.TempDir(), time.Hour, 5*time.Second, store, nil)
	if got := names(c.Dependencies(lang.Target{Ecosystem: Pub, Package: "billing", Version: "1.0.0"})); !slices.Equal(got, []string{"ledger"}) {
		t.Errorf("billing: %v (asked %v)", got, asked)
	}
	if store.Authorizes(server.URL + "/other/api/packages/billing") {
		t.Error("the token covers another path")
	}
}
