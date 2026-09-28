package index

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// A .bazelrc's --credential_helper is recorded by scope and never run: the most
// specific helper covering a registry's host is the one noted, a scope `*.x`
// covers x and every host below it, an unscoped helper covers every host, and
// --credential_helper_timeout is no helper. The repository's helpers go with
// the repository.
//
// Verifies: REQ-BAZEL-011
func TestBazelCredentialHelpers(t *testing.T) {
	files := write(t, map[string]string{
		"MODULE.bazel": "module(name = \"app\")\n",
		".bazelrc": "common --credential_helper=*.corp.test=%workspace%/tools/cred\n" +
			"build --credential_helper registry.corp.test=/usr/local/bin/registry-helper\n" +
			"common --credential_helper_timeout=5s\n",
	})
	home := t.TempDir()
	d := NewDiscoverer(environment(nil), home)
	config := d.Discover(files)
	for host, want := range map[string]string{
		"registry.corp.test": "registry.corp.test",
		"a.b.corp.test":      "*.corp.test",
		"corp.test":          "*.corp.test",
		"REGISTRY.corp.test": "registry.corp.test",
		"bcr.bazel.build":    "none",
		"xcorp.test":         "none",
	} {
		scope, ok := config.bazelHelperFor(host)
		if !ok {
			scope = "none"
		}
		if scope != want {
			t.Errorf("%s: helper %q, want %q", host, scope, want)
		}
	}
	if config = d.Discover(nil); len(config.bazelHelpers) != 0 {
		t.Errorf("the repository's helpers outlive it: %v", config.bazelHelpers)
	}
	put(t, filepath.Join(home, ".bazelrc"), "common --credential_helper=\"/usr/local/bin/any\"\n")
	if scope, ok := Discover(nil, environment(nil), home).bazelHelperFor("bcr.bazel.build"); !ok || scope != "" {
		t.Errorf("unscoped helper: %q %v", scope, ok)
	}
}

// A Bazel registry that a .bazelrc names a credential helper for is noted as
// asked without the helper's credentials, whether the answer came from the
// registry or from the cache; a registry no helper covers is not.
//
// Verifies: REQ-BAZEL-011, REQ-TRC-017
func TestBazelCredentialHelperIsNoted(t *testing.T) {
	var mu sync.Mutex
	asked := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked++
		mu.Unlock()
		w.Write([]byte("module(name = \"rules_go\")\nbazel_dep(name = \"platforms\", version = \"0.0.4\")\n"))
	}))
	t.Cleanup(server.Close)
	home := t.TempDir()
	put(t, filepath.Join(home, ".bazelrc"), "common --registry="+server.URL+"\ncommon --credential_helper=127.0.0.1=/opt/helper\n")
	directory := t.TempDir()
	target := lang.Target{Ecosystem: Bazel, Package: "rules_go", Version: "0.50.1", Pinned: true}
	for i := range 2 { // the second client answers from the first one's cache
		c := NewClient(Discover(nil, environment(nil), home), directory, time.Hour, 5*time.Second, auth.Read(home, nil), nil)
		got := notesOf(t, c, target)
		if len(got) != 1 || !strings.HasPrefix(got[0], trace.NoteHelperNotRun+": a .bazelrc names a credential helper for 127.0.0.1") ||
			!strings.Contains(got[0], server.URL) {
			t.Errorf("run %d: notes %q", i, got)
		}
	}
	if asked != 1 {
		t.Errorf("registry asked %d times", asked)
	}
	put(t, filepath.Join(home, ".bazelrc"), "common --registry="+server.URL+"\ncommon --credential_helper=*.corp.test=/opt/helper\n")
	c := NewClient(Discover(nil, environment(nil), home), t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, nil), nil)
	if got := notesOf(t, c, target); len(got) != 0 {
		t.Errorf("a helper for another host is noted: %q", got)
	}
}
