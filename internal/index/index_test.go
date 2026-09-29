package index

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
		absolute := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		out = append(out, &scan.File{Path: name, AbsolutePath: absolute})
	}
	return out
}

func environment(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// Verifies: REQ-SUP-014
func TestPublicDefaults(t *testing.T) {
	c := Discover(nil, environment(nil), "")
	for ecosystem, want := range map[string]string{
		NPM: "https://registry.npmjs.org", PyPI: "https://pypi.org/simple", Go: "https://proxy.golang.org",
	} {
		index, known := c.For(ecosystem, "anything")
		if index != want || !known {
			t.Errorf("%s: got %s (known %v), want %s known", ecosystem, index, known, want)
		}
	}
}

// Verifies: REQ-SUP-018
func TestRepositoryIndexIsNotVouchedFor(t *testing.T) {
	files := write(t, map[string]string{
		".npmrc": "registry=https://artifactory.internal/api/npm/all\n@acme:registry=https://artifactory.internal/api/npm/acme\n",
	})
	c := Discover(files, environment(nil), "")

	// The repository's index is what a package resolves from - and nothing on this
	// machine says it should be, which is the whole point of showing it.
	index, known := c.For(NPM, "lodash")
	if index != "https://artifactory.internal/api/npm/all" || known {
		t.Errorf("got %s (known %v), want the repository's index, unknown", index, known)
	}
	if index, _ := c.For(NPM, "@acme/tool"); index != "https://artifactory.internal/api/npm/acme" {
		t.Errorf("scoped package resolves from %s", index)
	}
	// A scope's index applies to that scope only.
	if index, _ := c.For(NPM, "@other/tool"); index != "https://artifactory.internal/api/npm/all" {
		t.Errorf("another scope resolves from %s", index)
	}
}

// Verifies: REQ-SUP-016
func TestMachineConfigurationWins(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".npmrc"), []byte("registry=https://mirror.corp/npm\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := write(t, map[string]string{".npmrc": "registry=https://somewhere.else/npm\n"})
	c := Discover(files, environment(nil), home)

	index, known := c.For(NPM, "lodash")
	if index != "https://mirror.corp/npm" || !known {
		t.Errorf("got %s (known %v), want this machine's mirror, known", index, known)
	}
}

// Verifies: REQ-SUP-016
func TestAScopedSourceBeatsTheMachinesUnscopedOne(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".npmrc"), []byte("registry=https://mirror.corp/npm\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := write(t, map[string]string{".npmrc": "@acme:registry=https://acme.example/npm\n"})
	c := Discover(files, environment(nil), home)

	// The repository's scope is the more specific answer, so it is the one recorded -
	// and, since only the repository names it, it is marked rather than fetched from.
	if index, known := c.For(NPM, "@acme/ui"); index != "https://acme.example/npm" || known {
		t.Errorf("@acme/ui: got %s (known %v), want the repository's scope, unknown", index, known)
	}
	if index, known := c.For(NPM, "lodash"); index != "https://mirror.corp/npm" || !known {
		t.Errorf("lodash: got %s (known %v), want this machine's mirror, known", index, known)
	}
}

// Paket names its feeds in paket.dependencies (per group) and paket.lock; a
// directory and nuget.org itself are not recorded.
//
// Verifies: REQ-FSHARP-010
func TestDiscoverReadsPaketFeeds(t *testing.T) {
	files := write(t, map[string]string{
		"paket.dependencies": "source https://www.nuget.org/api/v2\nsource ./local-packages\nnuget Argu\n\ngroup Build\n  source https://nuget.pkg.example.com/acme/index.json username: \"x\" password: \"%TOKEN%\"\n  nuget FAKE\n",
		"sub/paket.lock":     "NUGET\n  remote: https://feed.internal/v3/index.json\n    Argu (6.1.1)\n",
	})
	c := Discover(files, environment(nil), "")
	var got []string
	for _, s := range c.Sources(NuGet) {
		got = append(got, s.URL)
	}
	if want := []string{"https://nuget.pkg.example.com/acme/index.json", "https://feed.internal/v3/index.json"}; strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("paket feeds %v, want %v", got, want)
	}
}

// Verifies: REQ-SUP-015
func TestDiscoverReadsEveryEcosystem(t *testing.T) {
	files := write(t, map[string]string{
		"requirements.txt":   "--extra-index-url https://pypi.internal/simple\nrequests\n",
		"pyproject.toml":     "[[tool.uv.index]]\nname = \"corp\"\nurl = \"https://uv.internal/simple\"\n",
		"NuGet.config":       `<configuration><packageSources><add key="corp" value="https://nuget.internal/v3/index.json" /></packageSources></configuration>`,
		"pom.xml":            `<project><repositories><repository><url>https://maven.internal/releases</url></repository></repositories></project>`,
		".cargo/config.toml": "[source.corp]\nregistry = \"https://crates.internal/index\"\n",
		".yarnrc.yml":        "npmRegistryServer: \"https://yarn.internal/npm\"\n",
		"composer.json": `{"repositories": [{"type": "vcs", "url": "https://github.com/acme/fork"},
			{"type": "composer", "url": "https://satis.internal"}, {"packagist.org": false}]}`,
	})
	c := Discover(files, environment(map[string]string{"GOPROXY": "https://goproxy.internal,direct"}), "")

	// What replaces the public default is what a package is attributed to; what is
	// asked beside it is not, since which packages it holds is not known offline.
	for _, test := range []struct{ ecosystem, want string }{
		{PyPI, "https://pypi.org/simple"},
		{NuGet, "https://api.nuget.org/v3/index.json"},
		{Maven, "https://repo.maven.apache.org/maven2"},
		// [source.corp] is not what crates.io is replaced with: it serves nothing.
		{Cargo, "https://crates.io"},
		{NPM, "https://yarn.internal/npm"},
		{Go, "https://goproxy.internal"},
		{Composer, "https://satis.internal"}, // Packagist is off; a VCS repository is not an index
	} {
		if index, _ := c.For(test.ecosystem, "pkg"); index != test.want {
			t.Errorf("%s: got %s, want %s", test.ecosystem, index, test.want)
		}
	}
	// Every source is recorded, and each additive one is a candidate before the
	// public default, in the same order on every run.
	for _, test := range []struct {
		ecosystem string
		want      []string
	}{
		{PyPI, []string{"https://uv.internal/simple?", "https://pypi.internal/simple?", "https://pypi.org/simple"}},
		{NuGet, []string{"https://nuget.internal/v3/index.json?", "https://api.nuget.org/v3/index.json"}},
		{Maven, []string{"https://maven.internal/releases?", "https://repo.maven.apache.org/maven2"}},
		{Go, []string{"https://goproxy.internal"}},
		{Composer, []string{"https://satis.internal?"}},
	} {
		if got := order(c, test.ecosystem, "pkg", ""); strings.Join(got, " ") != strings.Join(test.want, " ") {
			t.Errorf("%s: asked in the order %v, want %v", test.ecosystem, got, test.want)
		}
	}
	// GOPROXY comes from this machine's environment, so it is vouched for; the
	// repository's own files are not.
	if _, known := c.For(Go, "example.com/mod"); !known {
		t.Error("GOPROXY from the environment is not trusted")
	}
	if _, known := c.For(Composer, "acme/lib"); known {
		t.Error("an index only the repository names is trusted")
	}
}

// order lists the indexes a package is asked of, in order, with "?" after one that
// is not fetched from.
func order(c *Config, ecosystem, packageName, registry string) []string {
	var out []string
	for _, k := range c.candidates(ecosystem, packageName, registry) {
		u := k.url
		if !k.known {
			u += "?"
		}
		out = append(out, u)
	}
	return out
}

// Verifies: REQ-SUP-017
func TestContainerRegistryComesFromTheReference(t *testing.T) {
	c := Discover(nil, environment(nil), "")
	for _, test := range []struct {
		image, want string
		known       bool
	}{
		{"nginx", "https://registry-1.docker.io", true},
		{"library/nginx", "https://registry-1.docker.io", true},
		{"ghcr.io/org/app", "https://ghcr.io", false},
		{"localhost:5000/app", "https://localhost:5000", false},
	} {
		index, known := c.For(OCI, test.image)
		if index != test.want || known != test.known {
			t.Errorf("%s: got %s (known %v), want %s (%v)", test.image, index, known, test.want, test.known)
		}
	}
}

// A registry this machine's container configuration names, or one the user vouched
// for, is asked about its images; one only the repository names is not.
//
// Verifies: REQ-SUP-026, REQ-SUP-042
func TestContainerRegistryTheMachineKnowsIsAsked(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".docker"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".docker", "config.json"), []byte(`{
		"auths": {"ghcr.io": {}},
		"credHelpers": {"123.dkr.ecr.eu-west-1.amazonaws.com": "ecr-login"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New()
	c.Credentials(auth.Read(home, nil))
	c.Trust([]string{"https://harbor.corp", "localhost:5000"})
	for _, test := range []struct {
		image string
		known bool
	}{
		{"ghcr.io/org/app", true},
		{"123.dkr.ecr.eu-west-1.amazonaws.com/app", true},
		{"harbor.corp/team/app", true},
		{"localhost:5000/app", true},
		{"quay.io/org/app", false},
	} {
		if _, known := c.For(OCI, test.image); known != test.known {
			t.Errorf("%s: known %v, want %v", test.image, known, test.known)
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

// Verifies: REQ-SUP-042
func TestAnIndexTheUserVouchesForIsNotMarked(t *testing.T) {
	// The shape this is for: an organization whose repositories carry their own
	// .npmrc pointing at the company registry. Without a word from the user that is
	// an index nothing on this machine configures, which is what dependency
	// confusion looks like - so every package is marked, and a warning that is
	// always on is a warning nobody reads.
	repositoryOnly := func() *Config {
		c := New()
		c.Add(NPM, Source{URL: "https://nexus.corp/repository/npm-group"})
		return c
	}

	c := repositoryOnly()
	if _, known := c.For(NPM, "@acme/widgets"); known {
		t.Error("an index only the repository names was trusted without being vouched for")
	}

	c = repositoryOnly()
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

// Verifies: REQ-SUP-038
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
//
// Verifies: REQ-AUTH-012, REQ-AUTH-013, REQ-TRC-002
func TestACredentialInAnIndexURLIsNotRecorded(t *testing.T) {
	store := auth.Read("", nil)
	config := New()
	config.Credentials(store)
	config.Add(PyPI, Source{URL: "https://deploy:s3cr3t@pypi.corp/simple", Trusted: true, Origin: OriginMachine})
	config.Add(NPM, Source{URL: "https://someone:else@npm.corp/", Origin: OriginProject})

	index, _ := config.For(PyPI, "requests")
	if index != "https://pypi.corp/simple" {
		t.Errorf("the index is recorded as %q", index)
	}
	for _, s := range config.Report() {
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

// A repository that names a Composer repository may not also supply its password:
// neither the auth.json beside its composer.json, nor the "config" section, nor a
// user:password in the repository's URL is sent to that host.
//
// Verifies: REQ-AUTH-012, REQ-AUTH-017
func TestARepositorysComposerCredentialsAreDiscarded(t *testing.T) {
	store := auth.Read(t.TempDir(), environment(nil))
	d := NewDiscoverer(environment(nil), "")
	d.Config().Credentials(store)
	config := d.Discover(write(t, map[string]string{
		"composer.json": `{"repositories": [{"type": "composer", "url": "https://satis.corp"},
			{"type": "composer", "url": "https://repo:leak@private.corp"}],
			"config": {"http-basic": {"satis.corp": {"username": "repo", "password": "leak"}}}}`,
		"auth.json": `{"http-basic": {"satis.corp": {"username": "repo", "password": "leak"}},
			"bearer": {"private.corp": "leak"}}`,
	}))
	var urls []string
	for _, s := range config.Report() {
		urls = append(urls, s.URL)
	}
	if !slices.Contains(urls, "https://satis.corp") || !slices.Contains(urls, "https://private.corp") {
		t.Fatalf("the repository's Composer repositories were not recorded: %v", urls)
	}
	for _, raw := range []string{"https://satis.corp/packages.json", "https://private.corp/packages.json"} {
		request, _ := http.NewRequest(http.MethodGet, raw, nil)
		store.Apply(request)
		if got := request.Header.Get("Authorization"); got != "" {
			t.Errorf("%s: a credential the repository supplied was sent: %q", raw, got)
		}
	}
}

// A repository that names a gem server may not also supply its password: neither
// the .bundle/config beside its Gemfile nor a user:password in a Gemfile source is
// sent to that host.
//
// Verifies: REQ-AUTH-012, REQ-AUTH-019
func TestARepositorysBundlerCredentialsAreDiscarded(t *testing.T) {
	store := auth.Read(t.TempDir(), environment(nil))
	d := NewDiscoverer(environment(nil), "")
	d.Config().Credentials(store)
	config := d.Discover(write(t, map[string]string{
		"Gemfile": "source \"https://gems.corp.test\"\n" +
			"source \"https://repo:leak@private.corp.test\" do\n  gem \"acme\"\nend\n",
		".bundle/config": "---\nBUNDLE_GEMS__CORP__TEST: \"repo:leak\"\nBUNDLE_PRIVATE__CORP__TEST: \"repo:leak\"\n",
	}))
	var urls []string
	for _, s := range config.Report() {
		urls = append(urls, s.URL)
	}
	if !slices.Contains(urls, "https://gems.corp.test") || !slices.Contains(urls, "https://private.corp.test") {
		t.Fatalf("the Gemfile's sources were not recorded: %v", urls)
	}
	for _, raw := range []string{"https://gems.corp.test/info/rack", "https://private.corp.test/info/acme"} {
		request, _ := http.NewRequest(http.MethodGet, raw, nil)
		store.Apply(request)
		if got := request.Header.Get("Authorization"); got != "" {
			t.Errorf("%s: a credential the repository supplied was sent: %q", raw, got)
		}
	}
}

// Under --watch the repository is read again on every analysis: an index taken out of
// it is gone from the next map, and one only this machine names stays.
//
// Verifies: REQ-SUP-015
func TestDiscoverForgetsWhatTheRepositoryNoLongerSays(t *testing.T) {
	d := NewDiscoverer(environment(map[string]string{"PIP_INDEX_URL": "https://pypi.machine/simple"}), "")
	files := write(t, map[string]string{".npmrc": "registry=https://npm.old/\n"})
	d.Discover(files)
	os.WriteFile(files[0].AbsolutePath, []byte("registry=https://npm.new/\n"), 0o644)
	c := d.Discover(files)

	var urls []string
	for _, s := range c.Sources(NPM) {
		urls = append(urls, s.URL)
	}
	if len(urls) != 1 || urls[0] != "https://npm.new" {
		t.Errorf("npm sources after the .npmrc changed: %v", urls)
	}
	if index, known := c.For(PyPI, "requests"); index != "https://pypi.machine/simple" || !known {
		t.Errorf("the machine's index was lost: %s (known %v)", index, known)
	}
}

// Cargo resolves crates.io from whatever replace-with ends at, and the answer must
// not depend on the order Go ranges over a map.
//
// Verifies: REQ-SUP-015
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
		parseCargoConfig([]byte(config), sink{put: func(ecosystem string, s Source) {
			if first == "" && s.URL != "" {
				first = s.URL
			}
		}})
		if first != "https://corp.example/index" {
			t.Fatalf("crates.io resolves from %s, want the source replace-with names", first)
		}
	}
}

// Composer's global config.json is read from COMPOSER_HOME and both usual homes, in
// the list and the object form of "repositories".
//
// Verifies: REQ-SUP-015
func TestReadsComposerConfigFromTheMachine(t *testing.T) {
	for _, directory := range [][]string{{".config", "composer"}, {".composer"}, {"custom-home"}} {
		home := t.TempDir()
		path := filepath.Join(append([]string{home}, directory...)...)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		config := `{"repositories": {"corp": {"type": "composer", "url": "https://packagist.corp"}, "packagist.org": false}}`
		if err := os.WriteFile(filepath.Join(path, "config.json"), []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
		variables := map[string]string{}
		if directory[0] == "custom-home" {
			variables["COMPOSER_HOME"] = path
		}
		c := Discover(nil, environment(variables), home)
		if index, known := c.For(Composer, "acme/billing"); index != "https://packagist.corp" || !known {
			t.Errorf("%s: got %s (known %v), want the machine's repository, known", filepath.Join(directory...), index, known)
		}
	}
}

// Verifies: REQ-SUP-015
func TestReadsNuGetConfigFromBothUserLocations(t *testing.T) {
	// The credentials are read from either file, so the feeds have to be as well, or
	// a feed kept only in the second has a password nothing ever uses.
	for _, directory := range [][]string{{".nuget", "NuGet"}, {".config", "NuGet"}} {
		home := t.TempDir()
		path := filepath.Join(append([]string{home}, directory...)...)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		config := `<configuration><packageSources><add key="corp" value="https://nuget.corp/v3/index.json" /></packageSources></configuration>`
		if err := os.WriteFile(filepath.Join(path, "NuGet.Config"), []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
		c := Discover(nil, environment(nil), home)
		want := []string{"https://nuget.corp/v3/index.json", "https://api.nuget.org/v3/index.json"}
		if got := order(c, NuGet, "Acme.Tools", ""); strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("~/%s: asked %v, want the machine's feed (known) before nuget.org", filepath.Join(directory...), got)
		}
	}
}

// A Gemfile's source serves every gem and a source block only the gems inside it; a
// second GEM remote of Gemfile.lock serves the gems locked under it.
//
// Verifies: REQ-SUP-015, REQ-SUP-045
func TestDiscoverReadsBundlerSources(t *testing.T) {
	files := write(t, map[string]string{
		"Gemfile": `source "https://gems.corp.test"

gem "rails"

source "https://private.corp.test" do
  gem "acme-auth"
end
gem "pg"
`,
		"engine/Gemfile.lock": `GEM
  remote: https://gems.corp.test/
  specs:
    rails (7.1.2)

GEM
  remote: https://vendor.corp.test/
  specs:
    vendor-kit (1.0.0)
      rails (>= 7)

DEPENDENCIES
  vendor-kit!
`,
	})
	c := Discover(files, environment(nil), "")
	for packageName, want := range map[string]string{
		"rails":      "https://gems.corp.test",
		"pg":         "https://gems.corp.test", // after the block: the global source again
		"acme-auth":  "https://private.corp.test",
		"vendor-kit": "https://vendor.corp.test",
	} {
		if index, known := c.For(RubyGems, packageName); index != want || known {
			t.Errorf("%s: got %s (known %v), want %s, unknown", packageName, index, known, want)
		}
	}
}

// Bundler's rubygems.org mirror (environment or ~/.bundle/config) and ~/.gemrc's
// sources are this machine's, and trusted.
//
// Verifies: REQ-SUP-015
func TestReadsRubyGemsConfigFromTheMachine(t *testing.T) {
	c := Discover(nil, environment(map[string]string{"BUNDLE_MIRROR__RUBYGEMS__ORG": "https://mirror.env.test"}), "")
	if index, known := c.For(RubyGems, "rack"); index != "https://mirror.env.test" || !known {
		t.Errorf("environment mirror: got %s (known %v)", index, known)
	}
	for file, content := range map[string]string{
		".gemrc":         "---\n:backtrace: false\n:sources:\n- https://gems.home.test/\n:update_sources: true\n",
		".bundle/config": "---\nBUNDLE_MIRROR__HTTPS://RUBYGEMS__ORG/: \"https://gems.home.test\"\n",
	} {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(home, file)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		c := Discover(nil, environment(nil), home)
		if index, known := c.For(RubyGems, "rack"); index != "https://gems.home.test" || !known {
			t.Errorf("%s: got %s (known %v), want the machine's source, known", file, index, known)
		}
	}
}

// A pubspec's hosted dependency names the server that serves it, and so does a
// package pubspec.lock resolved from somewhere other than pub.dev; PUB_HOSTED_URL is
// this machine's, and trusted.
//
// Verifies: REQ-SUP-015, REQ-SUP-046
func TestDiscoverReadsPubServers(t *testing.T) {
	files := write(t, map[string]string{
		"app/pubspec.yaml": `name: app
dependencies:
  http: ^1.2.0
  acme_auth:
    hosted: https://pub.corp.test
    version: ^1.0.0
dev_dependencies:
  acme_lints:
    hosted:
      name: acme_lints
      url: https://lints.corp.test
`,
		"app/pubspec.lock": `packages:
  http:
    dependency: "direct main"
    description:
      name: http
      url: "https://pub.dev"
    source: hosted
    version: "1.2.1"
  vendor_kit:
    dependency: transitive
    description:
      name: vendor_kit
      url: "https://vendor.corp.test/"
    source: hosted
    version: "2.0.0"
`,
	})
	c := Discover(files, environment(nil), "")
	for packageName, want := range map[string]string{
		"acme_auth":  "https://pub.corp.test",
		"acme_lints": "https://lints.corp.test",
		"vendor_kit": "https://vendor.corp.test",
	} {
		if index, known := c.For(Pub, packageName); index != want || known {
			t.Errorf("%s: got %s (known %v), want %s, unknown", packageName, index, known, want)
		}
	}
	if index, known := c.For(Pub, "http"); index != "https://pub.dev" || !known {
		t.Errorf("http: got %s (known %v), want pub.dev", index, known)
	}
	c = Discover(nil, environment(map[string]string{"PUB_HOSTED_URL": "https://pub.mirror.test"}), "")
	if index, known := c.For(Pub, "http"); index != "https://pub.mirror.test" || !known {
		t.Errorf("PUB_HOSTED_URL: got %s (known %v)", index, known)
	}
}

// HEX_API_URL is the Hex API this machine's Mix and rebar3 use, and trusted;
// without it, hex.pm's.
//
// Verifies: REQ-SUP-015, REQ-SUP-047
func TestDiscoverReadsHexAPI(t *testing.T) {
	if index, known := Discover(nil, environment(nil), "").For(Hex, "plug"); index != "https://hex.pm/api" || !known {
		t.Errorf("default: got %s (known %v)", index, known)
	}
	c := Discover(nil, environment(map[string]string{"HEX_API_URL": "https://hex.corp.test/api", "HEX_MIRROR": "https://mirror.test"}), "")
	if index, known := c.For(Hex, "plug"); index != "https://hex.corp.test/api" || !known {
		t.Errorf("HEX_API_URL: got %s (known %v)", index, known)
	}
}

// renv.lock's repositories are the repository's, asked in order, each also scoped
// to the packages installed from it; CRAN and Posit Package Manager are the public
// index. An options(repos = ...) in the project's .Rprofile is the repository's as
// well; in the user's ~/.Rprofile, R_PROFILE_USER or RENV_CONFIG_REPOS_OVERRIDE it
// is this machine's. A list that does not name CRAN switches it off.
//
// Verifies: REQ-SUP-015, REQ-SUP-048, REQ-SUP-063
func TestDiscoverReadsRRepositories(t *testing.T) {
	files := write(t, map[string]string{
		"renv.lock": `{"R": {"Version": "4.4.1", "Repositories": [
  {"Name": "CRAN", "URL": "https://cloud.r-project.org"},
  {"Name": "PPM", "URL": "https://packagemanager.posit.co/cran/latest"},
  {"Name": "internal", "URL": "https://cran.corp.test/latest"}]},
 "Packages": {
  "dplyr": {"Package": "dplyr", "Version": "1.1.4", "Source": "Repository", "Repository": "PPM"},
  "acmeR": {"Package": "acmeR", "Version": "0.3.0", "Source": "Repository", "Repository": "internal"}}}`,
		"sub/.Rprofile": `options(repos = c(CRAN = "https://cran.rstudio.com", drat = "https://acme.github.io/drat"))`,
	})
	c := Discover(files, environment(nil), "")
	for packageName, want := range map[string][]string{
		"acmeR": {"https://cran.corp.test/latest?"},
		"dplyr": {"https://cloud.r-project.org", "https://cran.corp.test/latest?", "https://acme.github.io/drat?"},
	} {
		if got := order(c, CRAN, packageName, ""); !slices.Equal(got, want) {
			t.Errorf("%s: asked %v, want %v", packageName, got, want)
		}
	}
	if got := order(Discover(nil, environment(nil), ""), CRAN, "dplyr", ""); !slices.Equal(got, []string{"https://cloud.r-project.org"}) {
		t.Errorf("default: %v", got)
	}

	home := t.TempDir()
	os.WriteFile(filepath.Join(home, ".Rprofile"), []byte(`local({
  options(repos = "https://r.corp.test")
})`), 0o644)
	if index, known := Discover(nil, environment(nil), home).For(CRAN, "dplyr"); index != "https://r.corp.test" || !known {
		t.Errorf("~/.Rprofile: got %s (known %v)", index, known)
	}
	if got := order(Discover(nil, environment(nil), home), CRAN, "dplyr", ""); !slices.Equal(got, []string{"https://r.corp.test"}) {
		t.Errorf("~/.Rprofile switches CRAN off: %v", got)
	}
	c = Discover(nil, environment(map[string]string{"RENV_CONFIG_REPOS_OVERRIDE": "CRAN=https://mirror.corp.test/cran"}), "")
	if got := order(c, CRAN, "dplyr", ""); !slices.Equal(got, []string{"https://mirror.corp.test/cran"}) {
		t.Errorf("RENV_CONFIG_REPOS_OVERRIDE: %v", got)
	}
	c = Discover(nil, environment(map[string]string{"RENV_CONFIG_REPOS_OVERRIDE": "https://mirror.corp.test/cran;CRAN=https://cloud.r-project.org"}), "")
	if got := order(c, CRAN, "dplyr", ""); !slices.Equal(got, []string{"https://mirror.corp.test/cran", "https://cloud.r-project.org"}) {
		t.Errorf("RENV_CONFIG_REPOS_OVERRIDE with CRAN: %v", got)
	}
	for _, test := range []struct{ profile, want string }{
		// The list extends the option it does not spell out: CRAN stays.
		{`options(repos = c(getOption("repos"), internal = "https://r.corp.test"))`, "https://r.corp.test https://cloud.r-project.org"},
		// "@CRAN@" is the CRAN mirror R asks for; its place in the list is kept.
		{`options("repos" = c(corp = "https://r.corp.test", CRAN = "@CRAN@"))`, "https://r.corp.test https://cloud.r-project.org"},
		// Bioconductor's repositories are left to the Bioconductor index, local ones
		// cannot be asked: neither is CRAN.
		{`options(repos = c(BioCsoft = "https://bioconductor.org/packages/3.18/bioc", local = "file:///srv/cran"))`, ""},
		{`options(repos = c(r = 'https://r.corp.test', "https://two.corp.test"), timeout = 60)`, "https://r.corp.test https://two.corp.test"},
	} {
		profile := filepath.Join(t.TempDir(), "profile.R")
		os.WriteFile(profile, []byte(test.profile), 0o644)
		c := Discover(nil, environment(map[string]string{"R_PROFILE_USER": profile}), "")
		if got := strings.Join(order(c, CRAN, "dplyr", ""), " "); got != test.want {
			t.Errorf("%s: asked %q, want %q", test.profile, got, test.want)
		}
	}
}

// Verifies: REQ-SUP-048
func TestCRANMirror(t *testing.T) {
	for u, want := range map[string]bool{
		"https://cloud.r-project.org":                                   true,
		"https://cran.r-project.org/":                                   true,
		"https://cran.rstudio.com":                                      true,
		"https://cran.uni-muenster.de":                                  false,
		"https://packagemanager.posit.co/cran/latest":                   true,
		"https://packagemanager.rstudio.com/all/__linux__/jammy/latest": true,
		"https://packagemanager.posit.co/bioconductor":                  false,
		"https://acme.r-universe.dev":                                   false,
		"https://cran.corp.test/latest":                                 false,
		"https://r.corp.test":                                           false,
	} {
		if got := CRANMirror(u); got != want {
			t.Errorf("%s: got %v, want %v", u, got, want)
		}
	}
}

// A repository stanza of cabal.project is the repository's index, asked before
// Hackage (cabal combines every repository's packages); Hackage itself is not
// recorded, and a mirror under Hackage's name replaces it. The configuration cabal
// reads - CABAL_CONFIG, CABAL_DIR's, ~/.cabal/config or the XDG one - is this
// machine's; one that lists repositories without Hackage leaves Hackage out.
//
// Verifies: REQ-SUP-015, REQ-SUP-049, REQ-SUP-063
func TestDiscoverReadsCabalRepositories(t *testing.T) {
	files := write(t, map[string]string{
		"cabal.project": "packages: .\n\nrepository hackage.haskell.org\n  url: http://hackage.haskell.org/\n\n" +
			"repository head.hackage.ghc.haskell.org\n   url: https://ghc.gitlab.haskell.org/head.hackage/\n   secure: True\n\n" +
			"repository local\n  url: file+noindex:///srv/packages\n",
	})
	if got := order(Discover(files, environment(nil), ""), Hackage, "aeson", ""); !slices.Equal(got, []string{"https://ghc.gitlab.haskell.org/head.hackage?", "https://hackage.haskell.org"}) {
		t.Errorf("cabal.project: %v", got)
	}
	if index, known := Discover(files, environment(nil), "").For(Hackage, "aeson"); index != "https://hackage.haskell.org" || !known {
		t.Errorf("cabal.project: attributed to %s (known %v)", index, known)
	}
	if got := order(Discover(nil, environment(nil), ""), Hackage, "aeson", ""); !slices.Equal(got, []string{"https://hackage.haskell.org"}) {
		t.Errorf("default: %v", got)
	}
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".cabal"), 0o755)
	os.WriteFile(filepath.Join(home, ".cabal", "config"), []byte("-- comment\nrepository hackage.haskell.org\n  url: https://hackage.mirror.corp.test/\n\nremote-repo-cache: /x\n"), 0o644)
	if got := order(Discover(nil, environment(nil), home), Hackage, "aeson", ""); !slices.Equal(got, []string{"https://hackage.mirror.corp.test"}) {
		t.Errorf("~/.cabal/config: %v", got)
	}
	directory := t.TempDir()
	os.WriteFile(filepath.Join(directory, "config"), []byte("repository corp\n  url: https://hackage.corp.test\n"), 0o644)
	if got := order(Discover(nil, environment(map[string]string{"CABAL_DIR": directory}), ""), Hackage, "aeson", ""); !slices.Equal(got, []string{"https://hackage.corp.test"}) {
		t.Errorf("CABAL_DIR: %v", got)
	}
	os.WriteFile(filepath.Join(directory, "config"), []byte("repository hackage.haskell.org\n  url: http://hackage.haskell.org/\nrepository corp\n  url: https://hackage.corp.test\n"), 0o644)
	if got := order(Discover(nil, environment(map[string]string{"CABAL_DIR": directory}), ""), Hackage, "aeson", ""); !slices.Equal(got, []string{"https://hackage.corp.test", "https://hackage.haskell.org"}) {
		t.Errorf("CABAL_DIR with Hackage: %v", got)
	}
}

// active-repositories chooses the repositories cabal asks and their order: its list
// searched last to first, ":rest" for every configured repository it does not name,
// ":none" for none. The repository's cabal.project (cabal.project.local over it)
// wins over this machine's configuration.
//
// Verifies: REQ-SUP-049, REQ-SUP-063
func TestCabalActiveRepositories(t *testing.T) {
	stanzas := "repository corp\n  url: https://hackage.corp.test\n\nrepository head\n  url: https://head.corp.test\n\n"
	for _, test := range []struct{ active, want string }{
		{"", "https://hackage.corp.test? https://head.corp.test? https://hackage.haskell.org"},
		{"active-repositories: hackage.haskell.org, corp:override\n", "https://hackage.corp.test? https://hackage.haskell.org"},
		{"active-repositories: :rest, corp\n", "https://hackage.corp.test? https://head.corp.test? https://hackage.haskell.org"},
		{"active-repositories:\n  , corp\n  , hackage.haskell.org\n  , head:merge\n", "https://head.corp.test? https://hackage.haskell.org https://hackage.corp.test?"},
		{"active-repositories: head\n", "https://head.corp.test?"},
		{"active-repositories: :none\n", ""},
	} {
		c := Discover(write(t, map[string]string{"cabal.project": "packages: .\n\n" + stanzas + test.active}), environment(nil), "")
		if got := strings.Join(order(c, Hackage, "aeson", ""), " "); got != test.want {
			t.Errorf("%q: asked %q, want %q", test.active, got, test.want)
		}
	}
	// This machine's list, until the repository's cabal.project.local sets another.
	config := filepath.Join(t.TempDir(), "config")
	os.WriteFile(config, []byte("repository hackage.haskell.org\n  url: http://hackage.haskell.org/\n\n"+stanzas+"active-repositories: corp\n"), 0o644)
	d := NewDiscoverer(environment(map[string]string{"CABAL_CONFIG": config}), "")
	if got := order(d.Discover(nil), Hackage, "aeson", ""); !slices.Equal(got, []string{"https://hackage.corp.test"}) {
		t.Errorf("machine: %v", got)
	}
	files := write(t, map[string]string{
		"cabal.project":       "packages: .\nactive-repositories: corp\n",
		"cabal.project.local": "active-repositories: hackage.haskell.org, head\n",
	})
	if got := order(d.Discover(files), Hackage, "aeson", ""); !slices.Equal(got, []string{"https://head.corp.test", "https://hackage.haskell.org"}) {
		t.Errorf("cabal.project.local: %v", got)
	}
	if got := order(d.Discover(nil), Hackage, "aeson", ""); !slices.Equal(got, []string{"https://hackage.corp.test"}) {
		t.Errorf("forgotten with the repository: %v", got)
	}
	// A configuration that leaves Hackage out leaves it out of ":rest" too.
	os.WriteFile(config, []byte(stanzas), 0o644)
	d = NewDiscoverer(environment(map[string]string{"CABAL_CONFIG": config}), "")
	if got := order(d.Discover(write(t, map[string]string{"cabal.project": "active-repositories: :rest\n"})), Hackage, "aeson", ""); !slices.Equal(got, []string{"https://head.corp.test", "https://hackage.corp.test"}) {
		t.Errorf(":rest without Hackage: %v", got)
	}
}

// A module named with its registry's host is served by that host, which is known when
// this machine's Terraform configuration names it or the user vouches for it; a
// module of the public registries has no host in its name.
//
// Verifies: REQ-SUP-050
func TestTerraformModuleRegistryFromTheName(t *testing.T) {
	home := t.TempDir()
	os.WriteFile(filepath.Join(home, ".terraformrc"), []byte("credentials \"tf.corp.test\" {\n  token = \"secret\"\n}\n"), 0o644)
	c := Discover(nil, environment(nil), home)
	c.Credentials(auth.Read(home, environment(nil)))
	if index, known := c.For(TerraformModule, "terraform-aws-modules/vpc/aws"); index != "https://registry.terraform.io" || !known {
		t.Errorf("public: got %s (known %v)", index, known)
	}
	if index, known := c.For(TerraformModule, "tf.corp.test/acme/vpc/aws//modules/x"); index != "https://tf.corp.test" || !known {
		t.Errorf("configured host: got %s (known %v)", index, known)
	}
	if index, known := c.For(TerraformModule, "tf.other.test/acme/vpc/aws"); index != "https://tf.other.test" || known {
		t.Errorf("unknown host: got %s (known %v)", index, known)
	}
	c.Trust([]string{"https://tf.other.test"})
	if _, known := c.For(TerraformModule, "tf.other.test/acme/vpc/aws"); !known {
		t.Error("--trust-index did not vouch for a Terraform registry host")
	}
}

// A Podfile's `source` lines and Podfile.lock's SPEC REPOS are the repository's spec
// repositories: a private one in the lock serves the pods installed from it, one only
// the Podfile names serves every pod (so none is named to the public CDN), and
// CocoaPods' own repository - the CDN, the trunk repository on GitHub - is the public
// index, never recorded.
//
// Verifies: REQ-SUP-015, REQ-SUP-051
func TestDiscoverReadsCocoaPodsSources(t *testing.T) {
	lock := "PODS:\n  - Acme/Core (1.0)\n  - AFNetworking (4.0.1)\n\nSPEC REPOS:\n" +
		"  https://github.com/acme/Specs.git:\n    - Acme/Core\n  trunk:\n    - AFNetworking\n"
	files := write(t, map[string]string{
		"ios/Podfile":      "source 'https://cdn.cocoapods.org/'\nsource 'https://github.com/CocoaPods/Specs.git'\npod 'AFNetworking'\n",
		"ios/Podfile.lock": lock,
	})
	config := Discover(files, environment(nil), "")
	if index, known := config.For(CocoaPods, "Acme"); index != "https://github.com/acme/Specs.git" || known {
		t.Errorf("Acme: got %s (known %v)", index, known)
	}
	if index, known := config.For(CocoaPods, "AFNetworking"); index != "https://cdn.cocoapods.org" || !known {
		t.Errorf("AFNetworking: got %s (known %v)", index, known)
	}
	files = write(t, map[string]string{"Podfile": "source 'https://git.corp.test/Specs.git'\nsource 'https://cdn.cocoapods.org/'\n"})
	if index, known := Discover(files, environment(nil), "").For(CocoaPods, "AFNetworking"); index != "https://git.corp.test/Specs.git" || known {
		t.Errorf("Podfile source: got %s (known %v)", index, known)
	}
	for repository, want := range map[string]bool{"trunk": true, "https://cdn.cocoapods.org/": true,
		"https://github.com/CocoaPods/Specs.git": true, "https://github.com/acme/Specs.git": false} {
		if got := CocoaPodsTrunk(repository); got != want {
			t.Errorf("%s: got %v, want %v", repository, got, want)
		}
	}
}

// A project's .luarocks/config-5.x.lua names the repository's rocks servers;
// LUAROCKS_CONFIG (which replaces the user's file) and ~/.luarocks/config-5.x.lua
// this machine's. rocks_servers replaces the default list: its servers are asked
// in order, luarocks.org only where it is listed, and a group's mirrors each after
// any failure of the one before.
//
// Verifies: REQ-SUP-015, REQ-SUP-052, REQ-SUP-063
func TestDiscoverReadsLuaRocksConfig(t *testing.T) {
	files := write(t, map[string]string{
		".luarocks/config-5.1.lua": `rocks_servers = { "https://luarocks.org", "https://rocks.corp.test/", "/srv/rocks" }`,
	})
	if got := order(Discover(files, environment(nil), ""), LuaRocks, "penlight", ""); !slices.Equal(got, []string{"https://luarocks.org", "https://rocks.corp.test?"}) {
		t.Errorf("project: %v", got)
	}
	if got := order(Discover(nil, environment(nil), ""), LuaRocks, "penlight", ""); !slices.Equal(got, []string{"https://luarocks.org"}) {
		t.Errorf("default: %v", got)
	}
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".luarocks"), 0o755)
	os.WriteFile(filepath.Join(home, ".luarocks", "config-5.4.lua"), []byte("rocks_servers = {\n  { 'https://mirror.corp.test', 'https://mirror2.corp.test' },\n  'https://rocks.corp.test',\n}\n"), 0o644)
	c := Discover(nil, environment(nil), home)
	if got := order(c, LuaRocks, "penlight", ""); !slices.Equal(got, []string{"https://mirror.corp.test", "https://mirror2.corp.test", "https://rocks.corp.test"}) {
		t.Errorf("home: %v", got)
	}
	var onError []bool
	for _, k := range c.candidates(LuaRocks, "penlight", "") {
		onError = append(onError, k.onError)
	}
	if !slices.Equal(onError, []bool{true, false, false}) {
		t.Errorf("a group's mirrors move on after any failure: %v", onError)
	}
	config := filepath.Join(t.TempDir(), "config.lua")
	os.WriteFile(config, []byte(`rocks_servers = { "https://rocks.env.test" }`), 0o644)
	if got := order(Discover(nil, environment(map[string]string{"LUAROCKS_CONFIG": config}), home), LuaRocks, "penlight", ""); !slices.Equal(got, []string{"https://rocks.env.test"}) {
		t.Errorf("LUAROCKS_CONFIG: %v", got)
	}
	if !LuaRocksItself("https://luarocks.org/dev") || LuaRocksItself("https://rocks.corp.test") {
		t.Error("LuaRocksItself")
	}
}

// Gradle scripts name their repositories in several notations; Maven Central and
// the repositories that serve Gradle's own plugins are not the project's, and a POM
// naming Central is no source of its own either.
//
// Verifies: REQ-SUP-015, REQ-SUP-056
func TestDiscoverReadsGradleRepositories(t *testing.T) {
	files := write(t, map[string]string{
		"settings.gradle.kts": `pluginManagement {
    repositories {
        gradlePluginPortal()
        maven { url = uri("https://plugins.internal/maven") }
    }
}
dependencyResolutionManagement {
    repositories {
        mavenCentral()
        google()
        maven("https://kts.internal/releases")
        maven { url = uri("https://repo1.maven.org/maven2") }
    }
}`,
		"lib/build.gradle": `buildscript { repositories { maven { url 'https://classpath.internal/' } } }
repositories {
    maven { url 'https://groovy.internal/maven' }
    maven {
        name = "corp"
        setUrl("https://seturl.internal/maven")
    }
    mavenLocal()
}`,
		"pom.xml": `<project><repositories><repository><url>https://repo.maven.apache.org/maven2</url></repository></repositories></project>`,
	})
	c := Discover(files, environment(nil), "")
	var got []string
	for _, s := range c.Sources(Maven) {
		got = append(got, s.URL)
	}
	want := []string{"https://groovy.internal/maven", "https://seturl.internal/maven", "https://kts.internal/releases"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("maven sources %v, want %v", got, want)
	}
}

// A credential says the user can reach a host, not that every feed on it is theirs:
// an index the repository names on a host this machine holds a credential for is
// still not known until the user vouches for it. Container images and Terraform
// registry modules name their host themselves, so there a credential does count.
//
// Verifies: REQ-SUP-043
func TestAMachineCredentialDoesNotVouchForARepositoryIndex(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".npmrc"), "//nexus.corp/:_authToken=machine-token\n")
	// cSpell: ignore dXNlcjpwYXNz
	put(t, filepath.Join(home, ".docker", "config.json"), `{"auths": {"ghcr.io": {"auth": "dXNlcjpwYXNz"}}}`)
	put(t, filepath.Join(home, ".terraform.d", "credentials.tfrc.json"), `{"credentials": {"tf.corp": {"token": "machine-token"}}}`)
	c := New()
	c.Credentials(auth.Read(home, environment(nil)))
	c.Add(NPM, Source{URL: "https://nexus.corp/repository/other-team"})
	if _, known := c.For(NPM, "@acme/widgets"); known {
		t.Error("a repository index was trusted because this machine holds a credential for its host")
	}
	if _, known := c.For(OCI, "ghcr.io/org/app"); !known {
		t.Error("an image on a registry this machine is logged in to was not known")
	}
	if _, known := c.For(TerraformModule, "tf.corp/acme/vpc/aws"); !known {
		t.Error("a module on a registry this machine holds a token for was not known")
	}
	c.Trust([]string{"https://nexus.corp/repository/other-team"})
	if _, known := c.For(NPM, "@acme/widgets"); !known {
		t.Error("the index was not known once the user vouched for it")
	}
}
