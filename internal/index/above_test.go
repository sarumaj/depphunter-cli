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
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// aboveTheCheckout lays out base/outer/checkout/project with checkout holding a
// .git, and returns outer (this machine's), checkout (the repository's) and project
// (the analyzed directory).
func aboveTheCheckout(t *testing.T) (outer, checkout, project string) {
	t.Helper()
	outer = filepath.Join(t.TempDir(), "outer")
	checkout = filepath.Join(outer, "checkout")
	project = filepath.Join(checkout, "project")
	for _, directory := range []string{filepath.Join(checkout, ".git"), project} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return outer, checkout, project
}

// scanned writes files under root and lists them as a scan of root would.
func scanned(t *testing.T, root string, files map[string]string) []*scan.File {
	t.Helper()
	var out []*scan.File
	for name, body := range files {
		absolute := filepath.Join(root, filepath.FromSlash(name))
		put(t, absolute, body)
		out = append(out, &scan.File{Path: name, AbsolutePath: absolute})
	}
	return out
}

// Yarn's files above the analyzed checkout are this machine's: their registries
// are trusted, and a scope's token there reaches the scope's registry. Those
// between the analyzed directory and the checkout's top are the repository's: a
// repository file merges with them key by key, so a token it binds to a variable
// goes to the registry, and with the npmAlwaysAuth, they give - once vouched for.
//
// Verifies: REQ-SUP-079, REQ-SUP-015, REQ-AUTH-023, REQ-AUTH-024
func TestYarnFilesAboveTheCheckout(t *testing.T) {
	corp := newNpmStub(t, "Bearer corp-secret")
	outer, checkout, project := aboveTheCheckout(t)
	put(t, filepath.Join(outer, ".yarnrc.yml"), "npmScopes:\n  corp:\n    npmRegistryServer: "+corp.URL+"/corp\n    npmAuthToken: \"${CORP_TOKEN}\"\n")
	put(t, filepath.Join(checkout, ".yarnrc.yml"), "npmRegistryServer: https://yarn.corp/npm\nnpmAlwaysAuth: true\n")
	files := scanned(t, project, map[string]string{"sub/.yarnrc.yml": "npmAuthToken: \"${NPM_TOKEN}\"\n"})
	e := environment(map[string]string{"CORP_TOKEN": "corp-secret", "NPM_TOKEN": "npm-secret"})
	home := t.TempDir()
	store := auth.ReadFor(home, project, e)
	d := NewDiscoverer(e, home)
	d.Root(project)
	d.Config().Credentials(store)
	d.Config().Trust([]string{"yarn.corp"})
	c := d.Discover(files)
	if got, want := sources(c, NPM), []string{"@corp " + corp.URL + "/corp", "https://yarn.corp/npm?"}; !slices.Equal(got, want) {
		t.Errorf("sources %v, want %v", got, want)
	}
	if got := authOf(store, "https://yarn.corp/npm/react/1.0.0"); got != "Bearer npm-secret" {
		t.Errorf("the repository's merged token: %q", got)
	}
	// The checkout's file between speaks for the analyzed directory even when the
	// scan found no Yarn file of its own.
	if got, want := sources(d.Discover(nil), NPM), []string{"@corp " + corp.URL + "/corp", "https://yarn.corp/npm?"}; !slices.Equal(got, want) {
		t.Errorf("without a scanned file: %v, want %v", got, want)
	}
	c = d.Discover(files)
	client := NewClient(c, t.TempDir(), time.Hour, 5*time.Second, store)
	if got := names(client.Dependencies(lang.Target{Ecosystem: NPM, Package: "@corp/ui", Version: "1.0.0"})); len(got) != 1 {
		t.Errorf("@corp/ui: %v, asked %v", got, corp.asked)
	}
}

// Without npmAlwaysAuth, Yarn's credential for a registry goes with the scoped
// packages' requests only: an unscoped package is asked without it, as Yarn asks.
// npmAlwaysAuth sends it with every request.
//
// Verifies: REQ-AUTH-024
func TestYarnAlwaysAuthEndToEnd(t *testing.T) {
	for _, always := range []bool{false, true} {
		registry := newNpmStub(t, "Bearer yarn-secret")
		home := t.TempDir()
		body := "npmRegistryServer: " + registry.URL + "\nnpmAuthToken: yarn-secret\n"
		if always {
			body += "npmAlwaysAuth: true\n"
		}
		put(t, filepath.Join(home, ".yarnrc.yml"), body)
		store := auth.Read(home, environment(nil))
		d := NewDiscoverer(environment(nil), home)
		d.Config().Credentials(store)
		c := NewClient(d.Discover(nil), t.TempDir(), time.Hour, 5*time.Second, store)
		if got := names(c.Dependencies(lang.Target{Ecosystem: NPM, Package: "@s/x", Version: "1.0.0"})); len(got) != 1 {
			t.Errorf("always %v, scoped: %v, asked %v", always, got, registry.asked)
		}
		got := names(c.Dependencies(lang.Target{Ecosystem: NPM, Package: "react", Version: "1.0.0"}))
		if (len(got) == 1) != always {
			t.Errorf("always %v, unscoped: %v, asked %v", always, got, registry.asked)
		}
	}
}

// NuGet's files, closest first: the project's, those between the analyzed directory
// and the checkout's top (the repository's), those above the checkout (this
// machine's), the user's NuGet.Config, the additional user files. Sources come out
// farthest first, each trusted as its file is.
//
// Verifies: REQ-SUP-065, REQ-SUP-079, REQ-SUP-064
func TestNuGetConfigsAboveTheCheckout(t *testing.T) {
	source := func(key string) string {
		return `<configuration><packageSources><add key="` + key + `" value="https://` + key + `.feed/v3/index.json"/></packageSources></configuration>`
	}
	outer, checkout, project := aboveTheCheckout(t)
	home := t.TempDir()
	put(t, filepath.Join(outer, "NuGet.Config"), source("outer"))
	put(t, filepath.Join(checkout, "NuGet.Config"), source("between"))
	userNuGet(t, home, `<packageSources><add key="user" value="https://user.feed/v3/index.json"/></packageSources>`)
	put(t, filepath.Join(home, ".nuget", "NuGet", "config", "extra.config"), source("extra"))
	files := scanned(t, project, map[string]string{"nuget.config": source("project")})
	d := NewDiscoverer(environment(nil), home)
	d.Root(project)
	c := d.Discover(files)
	var got []string
	for _, u := range order(c, NuGet, "Acme.Tools", "") {
		u, untrusted := strings.CutSuffix(u, "?")
		u = strings.TrimSuffix(strings.TrimPrefix(u, "https://"), ".feed/v3/index.json")
		if untrusted {
			u += "?"
		}
		got = append(got, u)
	}
	want := []string{"extra", "user", "outer", "between?", "project?", "api.nuget.org/v3/index.json"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
