package index

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// rebar3Client writes rebar3's global rebar.config and hex.config (when not empty)
// in a home directory and returns a client whose Hex API is the stub's, with the
// environment variables on top.
func rebar3Client(t *testing.T, stub *hexStub, global, hexConfig string, variables map[string]string) *Client {
	t.Helper()
	home := t.TempDir()
	directory := filepath.Join(home, ".config", "rebar3")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"rebar.config": global, "hex.config": hexConfig} {
		if body == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	all := map[string]string{"HEX_API_URL": stub.URL + "/api"}
	for k, v := range variables {
		all[k] = v
	}
	return NewClient(Discover(nil, environment(all), home), t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, environment(all)))
}

// paths are the stub's requests since before, without their Authorization.
func paths(stub *hexStub, before int) []string {
	var out []string
	for _, line := range stub.requests()[before:] {
		p, _, _ := strings.Cut(line, " ")
		out = append(out, p)
	}
	return out
}

// A rebar3 project's Hex package is asked of the repositories rebar3 asks, in its
// order: the project's organizations ({hex, [{repos, ...}]}), then those of the
// machine's global rebar.config, then hex.pm's public packages - found in the
// first, it is named to none after it, so hex.pm's impostor "billing" is never
// asked. A package no organization has is asked of hex.pm, as rebar3 would fetch
// it from there; with {repos, replace, [...]} it is not. Without a key for an
// organization the question stops there (ReasonNoKey) and nothing after it is
// asked; a repository with no API to ask ends the list the same way. A project
// with no repositories of its own ("*") on a machine without any is asked as
// before.
//
// Verifies: REQ-BEAM-013, REQ-SUP-047, REQ-AUTH-028
func TestRebar3OrganizationRepositories(t *testing.T) {
	stub := newHexStub(t, "user-key")
	key := map[string]string{"HEX_API_KEY": "user-key"}
	global := `{hex, [{repos, [#{name => <<"hexpm:beta">>}]}]}.` + "\n"
	c := rebar3Client(t, stub, global, "", key)

	billing := lang.Target{Ecosystem: Hex, Package: "billing", Version: "1.2.0", Registry: "hexpm:acme,*"}
	dependencies := c.Dependencies(billing)
	if got := names(dependencies); !slices.Equal(got, []string{"jason", "ledger"}) {
		t.Fatalf("billing: %v (asked %v)", got, stub.requests())
	}
	for _, d := range dependencies {
		if d.Registry != billing.Registry {
			t.Errorf("%s: registry %q", d.Package, d.Registry)
		}
	}
	if got := paths(stub, 0); !slices.Equal(got, []string{"/api/repos/acme/packages/billing", "/api/repos/acme/packages/billing/releases/1.2.0"}) {
		t.Errorf("billing asked %v", got)
	}

	// Not an organization's: acme, then beta, then hex.pm - where the key does not go.
	before := len(stub.requests())
	jason := lang.Target{Ecosystem: Hex, Package: "jason", Version: "1.2.0", Registry: "hexpm:acme,*"}
	if _, l := ask(t, c, jason); l.Index != stub.URL+"/api" || l.Reason != "" {
		t.Errorf("jason: %s (%s)", l.Index, l.Reason)
	}
	if got := paths(stub, before); !slices.Equal(got, []string{"/api/repos/acme/packages/jason", "/api/repos/beta/packages/jason",
		"/api/packages/jason", "/api/packages/jason/releases/1.2.0"}) {
		t.Errorf("jason asked %v", got)
	}
	for _, line := range stub.requests() {
		if p, k, _ := strings.Cut(line, " "); strings.HasPrefix(p, "/api/packages/") && k != "" {
			t.Errorf("the key went to a public package: %s", p)
		}
	}

	// The machine's repositories alone replace hex.pm's.
	c = rebar3Client(t, stub, `{hex, [{repos, replace, [#{name => <<"hexpm:acme">>}]}]}.`, "", key)
	before = len(stub.requests())
	if got, l := ask(t, c, lang.Target{Ecosystem: Hex, Package: "ledger", Version: "2.0.0", Registry: "*"}); len(got) != 0 || l.Index != stub.URL+"/api/repos/acme" {
		t.Errorf("ledger: %v from %s", got, l.Index)
	}
	if got := paths(stub, before); !slices.Equal(got, []string{"/api/repos/acme/packages/ledger"}) {
		t.Errorf("replaced: asked %v", got)
	}

	// No key: nothing after the organization is asked, and the report says why.
	for _, testCase := range []struct {
		registry, packageName string
		asked                 []string
	}{
		{"hexpm:acme,*", "billing", nil},
		{"hexpm:acme,*", "jason", nil},
		{"hexpm,hexpm:acme", "ghost", []string{"/api/packages/ghost"}},
	} {
		c = rebar3Client(t, stub, global, "", nil)
		before = len(stub.requests())
		if got, l := ask(t, c, lang.Target{Ecosystem: Hex, Package: testCase.packageName, Version: "1.2.0", Registry: testCase.registry}); len(got) != 0 ||
			l.Reason != trace.ReasonNoKey || l.Index != stub.URL+"/api/repos/acme" {
			t.Errorf("%s %s without a key: %v, %q at %s", testCase.registry, testCase.packageName, got, l.Reason, l.Index)
		}
		if got := paths(stub, before); !slices.Equal(got, testCase.asked) {
			t.Errorf("%s %s without a key: asked %v", testCase.registry, testCase.packageName, got)
		}
	}

	// A repository with no API ends the list.
	c = rebar3Client(t, stub, "", "", key)
	before = len(stub.requests())
	if _, l := ask(t, c, lang.Target{Ecosystem: Hex, Package: "jason", Version: "1.2.0", Registry: "mini,*"}); l.Reason != trace.ReasonNoIndex {
		t.Errorf("mini first: %q", l.Reason)
	}
	if _, l := ask(t, c, lang.Target{Ecosystem: Hex, Package: "ghost", Version: "1.2.0", Registry: "hexpm:acme,mini,*"}); l.Index != stub.URL+"/api/repos/acme" {
		t.Errorf("acme then mini: %s (%s)", l.Index, l.Reason)
	}
	if got := paths(stub, before); !slices.Equal(got, []string{"/api/repos/acme/packages/ghost"}) {
		t.Errorf("past a repository with no API: asked %v", got)
	}

	// Nothing configured anywhere: hex.pm's, exactly as a package without a Registry.
	c = rebar3Client(t, stub, "", "", nil)
	before = len(stub.requests())
	if got, l := ask(t, c, lang.Target{Ecosystem: Hex, Package: "jason", Version: "1.2.0", Registry: "*"}); len(got) != 0 || l.Index != stub.URL+"/api" || l.Reason != "" {
		t.Errorf("plain rebar3: %v from %s (%s)", got, l.Index, l.Reason)
	}
	if got := paths(stub, before); !slices.Equal(got, []string{"/api/packages/jason", "/api/packages/jason/releases/1.2.0"}) {
		t.Errorf("plain rebar3: asked %v", got)
	}
}

// The repo_key `rebar3 hex organization auth hexpm:acme` writes to rebar3's
// hex.config reaches the organization end to end.
//
// Verifies: REQ-AUTH-028, REQ-SUP-047
func TestRebar3OrganizationKeyFromHexConfig(t *testing.T) {
	stub := newHexStub(t, "acme-key")
	c := rebar3Client(t, stub, "", `#{<<"hexpm:acme">> => #{name => <<"hexpm:acme">>, repo_key => <<"acme-key">>}}.`, nil)
	if got := names(c.Dependencies(lang.Target{Ecosystem: Hex, Package: "billing", Version: "1.2.0", Registry: "hexpm:acme,*"})); !slices.Equal(got, []string{"jason", "ledger"}) {
		t.Errorf("billing: %v (asked %v)", got, stub.requests())
	}
	for _, line := range stub.requests() {
		if strings.HasPrefix(line, "/api/packages/") {
			t.Errorf("asked of the public side: %s", line)
		}
	}
}
