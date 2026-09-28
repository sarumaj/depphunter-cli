package index

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// notesOf asks c about each target with a fresh report and returns what the report
// noted, as "code: message" lines.
func notesOf(t *testing.T, c *Client, targets ...lang.Target) []string {
	t.Helper()
	report := trace.New(1, true, nil, nil)
	c.Trace(report)
	report.Enter("beam", 0)
	for _, target := range targets {
		c.Dependencies(target)
	}
	report.Finish()
	var out []string
	for _, n := range report.Notes {
		if n.Plugin != "beam" || n.File != "" {
			t.Errorf("note filed as %+v", n)
		}
		out = append(out, n.Code+": "+n.Message)
	}
	return out
}

// A Hex organization this machine has no key for is noted once, however many of
// its packages are asked about, with what provides a key.
//
// Verifies: REQ-SUP-047, REQ-TRC-017
func TestHexNoKeyIsNoted(t *testing.T) {
	stub := newHexStub(t, "user-key")
	home := t.TempDir()
	variables := map[string]string{"HEX_API_URL": stub.URL + "/api"}
	c := NewClient(Discover(nil, environment(variables), home), t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, environment(variables)), nil)
	got := notesOf(t, c,
		lang.Target{Ecosystem: Hex, Package: "billing", Version: "1.2.0", Registry: "hexpm:acme"},
		lang.Target{Ecosystem: Hex, Package: "ledger", Version: "2.0.0", Registry: "hexpm:acme"},
		lang.Target{Ecosystem: Hex, Package: "jason", Version: "1.2.0"})
	if len(got) != 1 || !strings.HasPrefix(got[0], trace.NoteNoKey+": no key on this machine for Hex organization acme ("+
		stub.URL+"/api/repos/acme)") || !strings.Contains(got[0], "HEX_REPOS_KEY") {
		t.Errorf("notes %q", got)
	}
}

// A key the organization's API refuses (403: an organization key holding only the
// repository permission) is noted with how to make one it accepts; a key it takes
// is not.
//
// Verifies: REQ-SUP-047, REQ-TRC-017
func TestHexForbiddenKeyIsNoted(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "repo-only" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(stub.Close)
	for key, want := range map[string]int{"repo-only": 1, "other": 0} {
		home := t.TempDir()
		variables := map[string]string{"HEX_API_URL": stub.URL + "/api", "HEX_API_KEY": key}
		c := NewClient(Discover(nil, environment(variables), home), t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, environment(variables)), nil)
		got := notesOf(t, c,
			lang.Target{Ecosystem: Hex, Package: "billing", Version: "1.2.0", Registry: "hexpm:acme"},
			lang.Target{Ecosystem: Hex, Package: "ledger", Version: "2.0.0", Registry: "hexpm:acme"})
		if len(got) != want {
			t.Fatalf("%s: notes %q", key, got)
		}
		if want == 1 && (!strings.HasPrefix(got[0], trace.NoteForbidden+": Hex organization acme refused the key") ||
			!strings.Contains(got[0], "api:read")) {
			t.Errorf("%s: notes %q", key, got)
		}
	}
}

// Under a packageSourceMapping, a package no pattern covers - which NuGet itself
// would refuse to restore - is noted; a mapped one, and any package without a
// mapping, is not.
//
// Verifies: REQ-SUP-065, REQ-TRC-017
func TestNuGetUnmappedIsNoted(t *testing.T) {
	feed := newNuGetStub(t, "", "", map[string]string{"Contoso.Billing": "Contoso.Core", "Newtonsoft.Json": "System.Runtime"})
	was := public[NuGet]
	public[NuGet] = feed.index()
	t.Cleanup(func() { public[NuGet] = was })
	home := t.TempDir()
	userNuGet(t, home, `<packageSources><add key="contoso" value="`+feed.index()+`"/></packageSources>
  <packageSourceMapping><packageSource key="contoso"><package pattern="Contoso.*"/></packageSource></packageSourceMapping>`)
	c := newClient(t, Discover(nil, environment(nil), home))
	got := notesOf(t, c,
		lang.Target{Ecosystem: NuGet, Package: "Contoso.Billing", Version: "1.0.0"},
		lang.Target{Ecosystem: NuGet, Package: "Newtonsoft.Json", Version: "1.0.0"})
	if len(got) != 1 || !strings.HasPrefix(got[0], trace.NoteUnmapped+": NuGet.Config's packageSourceMapping covers no pattern of Newtonsoft.Json") {
		t.Errorf("notes %q", got)
	}

	userNuGet(t, home, `<packageSources><add key="contoso" value="`+feed.index()+`"/></packageSources>`)
	c = newClient(t, Discover(nil, environment(nil), home))
	if got := notesOf(t, c, lang.Target{Ecosystem: NuGet, Package: "Newtonsoft.Json", Version: "1.0.0"}); len(got) != 0 {
		t.Errorf("without a mapping: notes %q", got)
	}
}
