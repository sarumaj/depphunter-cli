package index

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/scan"

	"github.com/sarumaj/depphunter-cli/internal/auth"
)

// write lays out a fixture and returns the scanned files, the way analysis sees them.
func write(t *testing.T, files map[string]string) []*scan.File {
	t.Helper()
	root := t.TempDir()
	var out []*scan.File
	for name, body := range files {
		abs := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		out = append(out, &scan.File{Path: name, Abs: abs})
	}
	return out
}

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestPublicDefaults(t *testing.T) {
	c := Discover(nil, env(nil), "")
	for eco, want := range map[string]string{
		NPM: "https://registry.npmjs.org", PyPI: "https://pypi.org/simple", Go: "https://proxy.golang.org",
	} {
		idx, known := c.For(eco, "anything")
		if idx != want || !known {
			t.Errorf("%s: got %s (known %v), want %s known", eco, idx, known, want)
		}
	}
}

func TestRepositoryIndexIsNotVouchedFor(t *testing.T) {
	files := write(t, map[string]string{
		".npmrc": "registry=https://artifactory.internal/api/npm/all\n@acme:registry=https://artifactory.internal/api/npm/acme\n",
	})
	c := Discover(files, env(nil), "")

	// The repository's index is what a package resolves from - and nothing on this
	// machine says it should be, which is the whole point of showing it.
	idx, known := c.For(NPM, "lodash")
	if idx != "https://artifactory.internal/api/npm/all" || known {
		t.Errorf("got %s (known %v), want the repository's index, unknown", idx, known)
	}
	if idx, _ := c.For(NPM, "@acme/tool"); idx != "https://artifactory.internal/api/npm/acme" {
		t.Errorf("scoped package resolves from %s", idx)
	}
	// A scope's index applies to that scope only.
	if idx, _ := c.For(NPM, "@other/tool"); idx != "https://artifactory.internal/api/npm/all" {
		t.Errorf("another scope resolves from %s", idx)
	}
}

func TestMachineConfigurationWins(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".npmrc"), []byte("registry=https://mirror.corp/npm\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := write(t, map[string]string{".npmrc": "registry=https://somewhere.else/npm\n"})
	c := Discover(files, env(nil), home)

	idx, known := c.For(NPM, "lodash")
	if idx != "https://mirror.corp/npm" || !known {
		t.Errorf("got %s (known %v), want this machine's mirror, known", idx, known)
	}
}

func TestDiscoverReadsEveryEcosystem(t *testing.T) {
	files := write(t, map[string]string{
		"requirements.txt":   "--extra-index-url https://pypi.internal/simple\nrequests\n",
		"pyproject.toml":     "[[tool.uv.index]]\nname = \"corp\"\nurl = \"https://uv.internal/simple\"\n",
		"NuGet.config":       `<configuration><packageSources><add key="corp" value="https://nuget.internal/v3/index.json" /></packageSources></configuration>`,
		"pom.xml":            `<project><repositories><repository><url>https://maven.internal/releases</url></repository></repositories></project>`,
		".cargo/config.toml": "[source.corp]\nregistry = \"https://crates.internal/index\"\n",
		".yarnrc.yml":        "npmRegistryServer: \"https://yarn.internal/npm\"\n",
	})
	c := Discover(files, env(map[string]string{"GOPROXY": "https://goproxy.internal,direct"}), "")

	for _, tt := range []struct{ eco, want string }{
		// Several files name an index; the first by path wins, the same way every run.
		{PyPI, "https://uv.internal/simple"},
		{NuGet, "https://nuget.internal/v3/index.json"},
		{Maven, "https://maven.internal/releases"},
		{Cargo, "https://crates.internal/index"},
		{NPM, "https://yarn.internal/npm"},
		{Go, "https://goproxy.internal"},
	} {
		if idx, _ := c.For(tt.eco, "pkg"); idx != tt.want {
			t.Errorf("%s: got %s, want %s", tt.eco, idx, tt.want)
		}
	}
	// The others are still recorded: the map shows one, the configuration keeps all.
	var found bool
	for _, s := range c.Sources(PyPI) {
		found = found || s.URL == "https://pypi.internal/simple"
	}
	if !found {
		t.Errorf("the requirements file's index was dropped: %+v", c.Sources(PyPI))
	}
	// GOPROXY comes from this machine's environment, so it is vouched for; the
	// repository's own files are not.
	if _, known := c.For(Go, "example.com/mod"); !known {
		t.Error("GOPROXY from the environment is not trusted")
	}
	if _, known := c.For(PyPI, "requests"); known {
		t.Error("an index only the repository names is trusted")
	}
}

func TestContainerRegistryComesFromTheReference(t *testing.T) {
	c := Discover(nil, env(nil), "")
	for _, tt := range []struct {
		image, want string
		known       bool
	}{
		{"nginx", "https://registry-1.docker.io", true},
		{"library/nginx", "https://registry-1.docker.io", true},
		{"ghcr.io/org/app", "https://ghcr.io", false},
		{"localhost:5000/app", "https://localhost:5000", false},
	} {
		idx, known := c.For(OCI, tt.image)
		if idx != tt.want || known != tt.known {
			t.Errorf("%s: got %s (known %v), want %s (%v)", tt.image, idx, known, tt.want, tt.known)
		}
	}
}

func TestHost(t *testing.T) {
	if got := Host("https://artifactory.internal/api/npm/all"); got != "artifactory.internal" {
		t.Errorf("got %q", got)
	}
	if got := Host("not a url"); got != "not a url" {
		t.Errorf("got %q", got)
	}
}

func TestAnIndexTheUserVouchesForIsNotMarked(t *testing.T) {
	// The shape this is for: an organization whose repositories carry their own
	// .npmrc pointing at the company registry. Without a word from the user that is
	// an index nothing on this machine configures, which is what dependency
	// confusion looks like - so every package is marked, and a warning that is
	// always on is a warning nobody reads.
	repoOnly := func() *Config {
		c := New()
		c.Add(NPM, Source{URL: "https://nexus.corp/repository/npm-group"})
		return c
	}

	c := repoOnly()
	if _, known := c.For(NPM, "@acme/widgets"); known {
		t.Error("an index only the repository names was trusted without being vouched for")
	}

	c = repoOnly()
	c.Trust([]string{"https://nexus.corp/repository/npm-group/"}) // a trailing slash is the same index
	index, known := c.For(NPM, "@acme/widgets")
	if index != "https://nexus.corp/repository/npm-group" || !known {
		t.Errorf("got %q known=%v after vouching for it", index, known)
	}
	// Vouching for one index says nothing about any other.
	c.Add(PyPI, Source{URL: "https://pypi.evil.example/simple"})
	if _, known := c.For(PyPI, "requests"); known {
		t.Error("vouching for one index trusted another")
	}
}

func TestPublicNamesTheEcosystemsOwnIndex(t *testing.T) {
	c := New()
	if !c.Public(NPM, "https://registry.npmjs.org") {
		t.Error("the public npm registry was not recognized as public")
	}
	if c.Public(NPM, "https://nexus.corp/repository/npm-group") {
		t.Error("a company registry was called public")
	}
	if c.Public(NPM, "") {
		t.Error("nothing at all was called public")
	}
}

// TestACredentialInAnIndexURLIsNotRecorded is the disclosure this guards against: the
// index a package resolves from is put on the graph, shown in the side panel and
// written into every export, and the HTML export is a file the documentation suggests
// sharing. A pip or Cargo mirror is routinely configured with the credential in the
// URL.
func TestACredentialInAnIndexURLIsNotRecorded(t *testing.T) {
	store := auth.Read("", nil)
	cfg := New()
	cfg.Credentials(store)
	cfg.Add(PyPI, Source{URL: "https://deploy:s3cr3t@pypi.corp/simple", Trusted: true, Origin: OriginMachine})
	cfg.Add(NPM, Source{URL: "https://someone:else@npm.corp/", Origin: OriginProject})

	index, _ := cfg.For(PyPI, "requests")
	if index != "https://pypi.corp/simple" {
		t.Errorf("the index is recorded as %q", index)
	}
	for _, s := range cfg.Report() {
		if strings.Contains(s.URL, "s3cr3t") || strings.Contains(s.URL, "else") {
			t.Errorf("the resolution report carries a credential: %q", s.URL)
		}
	}
	// This machine's own configuration supplied one, so it is kept and sent to that
	// host; the repository's is discarded rather than kept for a host it chose.
	ours, _ := http.NewRequest(http.MethodGet, "https://pypi.corp/simple/requests/", nil)
	store.Apply(ours)
	if ours.Header.Get("Authorization") == "" {
		t.Error("the credential the machine supplied was not kept")
	}
	theirs, _ := http.NewRequest(http.MethodGet, "https://npm.corp/react", nil)
	store.Apply(theirs)
	if got := theirs.Header.Get("Authorization"); got != "" {
		t.Errorf("a credential the repository supplied was sent: %q", got)
	}
}

// Under --watch the repository is read again on every analysis: an index taken out of
// it is gone from the next map, and one only this machine names stays.
func TestDiscoverForgetsWhatTheRepositoryNoLongerSays(t *testing.T) {
	d := NewDiscoverer(env(map[string]string{"PIP_INDEX_URL": "https://pypi.machine/simple"}), "")
	files := write(t, map[string]string{".npmrc": "registry=https://npm.old/\n"})
	d.Discover(files)
	os.WriteFile(files[0].Abs, []byte("registry=https://npm.new/\n"), 0o644)
	c := d.Discover(files)

	var urls []string
	for _, s := range c.Sources(NPM) {
		urls = append(urls, s.URL)
	}
	if len(urls) != 1 || urls[0] != "https://npm.new" {
		t.Errorf("npm sources after the .npmrc changed: %v", urls)
	}
	if idx, known := c.For(PyPI, "requests"); idx != "https://pypi.machine/simple" || !known {
		t.Errorf("the machine's index was lost: %s (known %v)", idx, known)
	}
}

// Cargo resolves crates.io from whatever replace-with ends at, and the answer must
// not depend on the order Go ranges over a map.
func TestCargoFollowsReplaceWith(t *testing.T) {
	config := `[source.crates-io]
replace-with = "vendored"
[source.vendored]
replace-with = "corp"
[source.aaa]
registry = "https://aaa.example/index"
[source.corp]
registry = "https://corp.example/index"
[registries.zzz]
index = "https://zzz.example/index"
`
	for range 20 {
		var first string
		parseCargoConfig([]byte(config), func(eco, url, scope string) {
			if first == "" && url != "" {
				first = url
			}
		})
		if first != "https://corp.example/index" {
			t.Fatalf("crates.io resolves from %s, want the source replace-with names", first)
		}
	}
}
