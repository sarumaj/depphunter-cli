package index

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// This machine's uv and PDM configuration names trusted PyPI sources: uv's
// variables over its user's uv.toml (UV_DEFAULT_INDEX winning over the file's
// default), PDM_PYPI_URL over PDM's [pypi] url. Explicit uv indexes and Poetry's
// repositories are not sources.
//
// Verifies: REQ-SUP-066, REQ-SUP-064
func TestPythonMachineIndexes(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "uv", "uv.toml"), `
[[index]]
name = "file-default"
url = "https://uv-file.corp/simple"
default = true

[[index]]
name = "file-extra"
url = "https://uv-extra.corp/simple"

[[index]]
name = "torch"
url = "https://torch.corp/whl"
explicit = true
`)
	put(t, filepath.Join(home, ".config", "pdm", "config.toml"),
		"[pypi]\nurl = \"https://pdm-file.corp/simple\"\n\n[pypi.pdmextra]\nurl = \"https://pdm-extra.corp/simple\"\n")
	put(t, filepath.Join(home, ".config", "pypoetry", "config.toml"), "[repositories.publish]\nurl = \"https://upload.corp/legacy\"\n")
	c := discoverOn(home, "linux", map[string]string{
		"UV_DEFAULT_INDEX": "https://uv-env.corp/simple",
		"UV_INDEX":         "envx=https://uv-env-extra.corp/simple",
		"PDM_PYPI_URL":     "https://pdm-env.corp/simple",
	})
	got := strings.Join(sources(c, PyPI), " ")
	want := "https://uv-env.corp/simple https://uv-env-extra.corp/simple https://uv-file.corp/simple " +
		"https://uv-extra.corp/simple https://pdm-env.corp/simple https://pdm-extra.corp/simple"
	if got != want {
		t.Errorf("sources\n got %s\nwant %s", got, want)
	}
	if got := order(c, PyPI, "requests", ""); !slices.Equal(got, []string{"https://uv-env-extra.corp/simple",
		"https://uv-extra.corp/simple", "https://pdm-extra.corp/simple", "https://uv-env.corp/simple"}) {
		t.Errorf("asked %v", got)
	}

	// UV_NO_CONFIG: uv's files are not read, its variables still are.
	c = discoverOn(home, "linux", map[string]string{"UV_NO_CONFIG": "1", "UV_INDEX": "https://uv-env-extra.corp/simple"})
	got = strings.Join(sources(c, PyPI), " ")
	if got != "https://uv-env-extra.corp/simple https://pdm-file.corp/simple https://pdm-extra.corp/simple" {
		t.Errorf("UV_NO_CONFIG: %s", got)
	}
}

// A directory's uv.toml replaces the [tool.uv] indexes of the pyproject.toml beside
// it. That pyproject.toml's [tool.uv.sources] still pins, as uv lowers it: against
// the indexes uv's variables name (such an index staying trusted), then the
// pyproject.toml's own [[tool.uv.index]] entries - never a uv.toml's, the user's
// included, which uv refuses to resolve a pin against. Without uv.toml, or under
// UV_NO_CONFIG, the pyproject.toml's indexes are read.
//
// Verifies: REQ-SUP-066
func TestUVTomlOverPyproject(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "uv", "uv.toml"), "[[index]]\nname = \"torch\"\nurl = \"https://torch.corp/whl\"\nexplicit = true\n")
	files := write(t, map[string]string{
		"svc/uv.toml": "[[index]]\nname = \"corp\"\nurl = \"https://uvtoml.corp/simple\"\ndefault = true\n\n" +
			"[[index]]\nname = \"only-in-uv-toml\"\nurl = \"https://only.corp/simple\"\n",
		"svc/pyproject.toml": "[[tool.uv.index]]\nname = \"corp\"\nurl = \"https://masked.corp/simple\"\n\n" +
			"[tool.uv.sources]\nacme = { index = \"corp\" }\ntorch = { index = \"torch\" }\nnone = { index = \"missing\" }\n" +
			"other = { index = \"only-in-uv-toml\" }\nshadowed = { index = \"env\" }\n\n" +
			"[[tool.uv.index]]\nname = \"env\"\nurl = \"https://shadowed.corp/simple\"\n",
		"lib/pyproject.toml": "[[tool.uv.index]]\nname = \"lib\"\nurl = \"https://lib.corp/simple\"\n",
	})
	c := discoverOn(home, "linux", map[string]string{"UV_INDEX": "env=https://env.corp/simple"})
	c.project(files)
	got := strings.Join(sources(c, PyPI), ", ")
	want := "https://env.corp/simple, https://lib.corp/simple?, acme https://masked.corp/simple?, shadowed https://env.corp/simple, " +
		"https://uvtoml.corp/simple?, https://only.corp/simple?"
	if got != want {
		t.Errorf("sources\n got %s\nwant %s", got, want)
	}
	for packageName, wantOrder := range map[string][]string{
		"torch":    {"https://env.corp/simple", "https://lib.corp/simple?", "https://only.corp/simple?", "https://uvtoml.corp/simple?"},
		"acme":     {"https://masked.corp/simple?"},
		"shadowed": {"https://env.corp/simple"},
	} {
		if got := order(c, PyPI, packageName, ""); !slices.Equal(got, wantOrder) {
			t.Errorf("%s asked of %v, want %v", packageName, got, wantOrder)
		}
	}

	c = discoverOn(home, "linux", map[string]string{"UV_NO_CONFIG": "1"})
	c.project(files)
	got = strings.Join(sources(c, PyPI), ", ")
	want = "https://lib.corp/simple?, https://masked.corp/simple?, https://shadowed.corp/simple?, acme https://masked.corp/simple?, " +
		"shadowed https://shadowed.corp/simple?"
	if got != want {
		t.Errorf("UV_NO_CONFIG\n got %s\nwant %s", got, want)
	}
}

// Poetry asks its primary sources in order (a default one first) and PyPI not at
// all; supplemental ones are asked after them, and an explicit one serves only the
// dependencies that name it. End to end: a package the first primary lacks is found
// on the second, and PyPI hears of nothing.
//
// Verifies: REQ-SUP-066, REQ-SUP-063
func TestSeveralPoetryPrimaries(t *testing.T) {
	pypi := newFeed(t, map[string]string{"/pypi/lib/1.0.0/json": pypiJSON("evil")})
	first := newFeed(t, nil)
	second := newFeed(t, map[string]string{"/pypi/lib/1.0.0/json": pypiJSON("certifi")})
	supplementary := newFeed(t, nil)
	explicit := newFeed(t, map[string]string{"/pypi/acme/1.0.0/json": pypiJSON("acme-core")})
	asPublic(t, PyPI, pypi)
	files := write(t, map[string]string{"pyproject.toml": `
[tool.poetry.dependencies]
acme = { version = "^1", source = "private" }

[[tool.poetry.source]]
name = "supp"
url = "` + supplementary.URL + `/simple"
priority = "supplemental"

[[tool.poetry.source]]
name = "first"
url = "` + first.URL + `/simple"

[[tool.poetry.source]]
name = "private"
url = "` + explicit.URL + `/simple"
priority = "explicit"

[[tool.poetry.source]]
name = "second"
url = "` + second.URL + `/simple"
priority = "primary"
`})
	d := NewDiscoverer(environment(nil), "")
	d.Config().Trust([]string{first.URL + "/simple", second.URL + "/simple", supplementary.URL + "/simple", explicit.URL + "/simple"})
	config := d.Discover(files)
	if got, want := order(config, PyPI, "lib", ""), []string{first.URL + "/simple", second.URL + "/simple", supplementary.URL + "/simple"}; !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
	c := newClient(t, config)
	got, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: "lib", Version: "1.0.0"})
	if !slices.Equal(got, []string{"certifi"}) || l.Index != second.URL+"/simple" {
		t.Errorf("lib: %v from %s", got, l.Index)
	}
	got, _ = ask(t, c, lang.Target{Ecosystem: PyPI, Package: "acme", Version: "1.0.0"})
	if !slices.Equal(got, []string{"acme-core"}) {
		t.Errorf("acme: %v", got)
	}
	if len(pypi.paths()) != 0 || first.askedFor("acme") || supplementary.askedFor("acme") || explicit.askedFor("lib") {
		t.Errorf("pypi %v, first %v, supp %v, explicit %v", pypi.paths(), first.paths(), supplementary.paths(), explicit.paths())
	}

	// A Poetry source named PyPI takes PyPI's place among the primaries.
	c2 := Discover(write(t, map[string]string{"pyproject.toml": "[[tool.poetry.source]]\nname = \"corp\"\nurl = \"https://corp.example/simple\"\n\n" +
		"[[tool.poetry.source]]\nname = \"PyPI\"\npriority = \"primary\"\n"}), environment(nil), "")
	if got := order(c2, PyPI, "x", ""); !slices.Equal(got, []string{"https://corp.example/simple?", pypi.URL}) {
		t.Errorf("with PyPI listed: %v", got)
	}
}

// Pipfile: the first source replaces PyPI, the others are asked beside it, and a
// package with index = "<name>" is served by that source alone; Pipfile.lock's
// sources and pins read the same. A source URL whose host is a variable is not
// recorded; its credentials never are.
//
// Verifies: REQ-SUP-066
func TestPipfileSources(t *testing.T) {
	c := Discover(write(t, map[string]string{
		"Pipfile": `
[[source]]
name = "corp"
url = "https://${USER}:${PASS}@pipenv.corp/simple"

[[source]]
name = "pypi"
url = "https://pypi.org/simple"

[[source]]
name = "hidden"
url = "https://${HIDDEN_HOST}/simple"

[packages]
acme = { version = "*", index = "corp" }
requests = "*"
`,
		"app/Pipfile.lock": `{"_meta": {"sources": [{"name": "lockcorp", "url": "https://lock.corp/simple"}]},
"default": {"billing": {"index": "lockcorp", "version": "==1"}}}`,
	}), environment(nil), "")
	got := strings.Join(sources(c, PyPI), ", ")
	want := "https://pipenv.corp/simple?, https://pypi.org/simple?, acme https://pipenv.corp/simple?, " +
		"https://lock.corp/simple?, billing https://lock.corp/simple?"
	if got != want {
		t.Errorf("sources\n got %s\nwant %s", got, want)
	}
	for packageName, wantOrder := range map[string][]string{
		"requests": {"https://pypi.org/simple", "https://pipenv.corp/simple?"},
		"acme":     {"https://pipenv.corp/simple?"},
		"Billing":  {"https://lock.corp/simple?"},
	} {
		if got := order(c, PyPI, packageName, ""); !slices.Equal(got, wantOrder) {
			t.Errorf("%s: %v, want %v", packageName, got, wantOrder)
		}
	}
}

// PDM: a source named pypi replaces PyPI, the others are asked beside it, a
// find_links source is no index, and include_packages patterns (globs, names
// compared as PEP 503 normalizes them) send the matching packages to their source
// alone. A project's pdm.toml names sources too.
//
// Verifies: REQ-SUP-066
func TestPDMSources(t *testing.T) {
	c := Discover(write(t, map[string]string{
		"pyproject.toml": `
[[tool.pdm.source]]
name = "private"
url = "https://pdm.corp/simple"
include_packages = ["acme-*"]

[[tool.pdm.source]]
name = "pypi"
url = "https://mirror.corp/simple"

[[tool.pdm.source]]
name = "wheels"
url = "https://wheels.corp/"
type = "find_links"
`,
		"pdm.toml": "[pypi.team]\nurl = \"https://team.corp/simple\"\n",
	}), environment(nil), "")
	for packageName, want := range map[string][]string{
		"requests":     {"https://team.corp/simple?", "https://pdm.corp/simple?", "https://mirror.corp/simple?"},
		"Acme_Utils":   {"https://pdm.corp/simple?"},
		"acme.billing": {"https://pdm.corp/simple?"},
	} {
		if got := order(c, PyPI, packageName, ""); !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", packageName, got, want)
		}
	}
}

// pythonRun discovers files with this machine's home and environment, vouching for
// trust, and answers the credential store.
func pythonRun(t *testing.T, home string, variables map[string]string, files map[string]string, trust ...string) *auth.Store {
	t.Helper()
	e := environment(variables)
	store := auth.Read(home, e)
	d := NewDiscoverer(e, home)
	d.Config().Credentials(store)
	d.Config().Trust(trust)
	d.Discover(write(t, files))
	return store
}

// A repository's Python index gets a credential this machine keeps by the index's
// name - UV_INDEX_<NAME>_*, Poetry's http-basic, PDM's [pypi.<name>] - or refers to
// with ${VAR} in its URL (Pipfile, PDM), only when this machine vouches for the
// index: --trust-index, or a PyPI index (or Poetry repository) of this machine's on
// its host. It serves the index's path; a secret written out in the repository is
// discarded; the machine's own credential is kept.
//
// Verifies: REQ-AUTH-023, REQ-AUTH-026
func TestRepositoryPythonCredentials(t *testing.T) {
	variables := map[string]string{
		"UV_INDEX_TEAM_USERNAME": "uv", "UV_INDEX_TEAM_PASSWORD": "uv-secret",
		"POETRY_HTTP_BASIC_CORP_PASSWORD": "poetry-secret", "POETRY_HTTP_BASIC_CORP_USERNAME": "poetry",
		"PIP_USER": "pipenv", "PIP_PASS": "pipenv-secret", "PDM_TOKEN": "pdm-token",
	}
	files := map[string]string{
		"pyproject.toml": `
[[tool.uv.index]]
name = "team"
url = "https://uv.corp/team/simple"

[[tool.poetry.source]]
name = "corp"
url = "https://poetry.corp/simple"

[[tool.pdm.source]]
name = "tok"
url = "https://${PDM_TOKEN}@pdm.corp/simple"

[[tool.pdm.source]]
name = "lit"
url = "https://ci:written@literal.corp/simple"
`,
		"Pipfile": "[[source]]\nname = \"p\"\nurl = \"https://${PIP_USER}:${PIP_PASS}@pipenv.corp/x/simple\"\n",
	}
	all := []string{"https://uv.corp/team/pypi/a/json", "https://poetry.corp/pypi/a/json",
		"https://pdm.corp/pypi/a/json", "https://literal.corp/pypi/a/json", "https://pipenv.corp/x/pypi/a/json"}

	s := pythonRun(t, t.TempDir(), variables, files)
	for _, u := range all {
		if got := authOf(s, u); got != "" {
			t.Errorf("unvouched %s got %q", u, got)
		}
	}

	s = pythonRun(t, t.TempDir(), variables, files, "uv.corp", "poetry.corp", "pdm.corp", "literal.corp", "https://pipenv.corp/x/simple")
	want := map[string]string{
		"https://uv.corp/team/pypi/a/json":  basicHeaderOf("uv:uv-secret"),
		"https://poetry.corp/pypi/a/json":   basicHeaderOf("poetry:poetry-secret"),
		"https://pdm.corp/pypi/a/json":      basicHeaderOf("pdm-token:"),
		"https://pipenv.corp/x/pypi/a/json": basicHeaderOf("pipenv:pipenv-secret"),
	}
	for _, u := range all {
		if got := authOf(s, u); got != want[u] {
			t.Errorf("vouched %s: %q, want %q", u, got, want[u])
		}
	}
	if got := authOf(s, "https://uv.corp/other/pypi/a/json"); got != "" {
		t.Errorf("the lent credential reached another path: %q", got)
	}

	// This machine's own indexes vouch for their hosts: a uv index (whose own
	// credential stays), a Poetry repository (not a source), a PDM source.
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "uv", "uv.toml"), "[[index]]\nname = \"mine\"\nurl = \"https://uv.corp/team/simple\"\n")
	put(t, filepath.Join(home, ".config", "pypoetry", "config.toml"), "[repositories.corp]\nurl = \"https://poetry.corp/upload\"\n")
	put(t, filepath.Join(home, ".config", "pdm", "config.toml"), "[pypi.p]\nurl = \"https://pdm.corp/other/simple\"\n")
	machineVariables := map[string]string{"UV_INDEX_MINE_USERNAME": "me", "UV_INDEX_MINE_PASSWORD": "mine"}
	for k, v := range variables {
		machineVariables[k] = v
	}
	s = pythonRun(t, home, machineVariables, files)
	want = map[string]string{
		"https://uv.corp/team/pypi/a/json": basicHeaderOf("me:mine"),
		"https://poetry.corp/pypi/a/json":  basicHeaderOf("poetry:poetry-secret"),
		"https://pdm.corp/pypi/a/json":     basicHeaderOf("pdm-token:"),
	}
	for _, u := range all {
		if got := authOf(s, u); got != want[u] {
			t.Errorf("machine host %s: %q, want %q", u, got, want[u])
		}
	}
}

// PDM's [pypi.<name>] credential goes to a repository source of the same name on a
// host this machine vouches for, and not to one elsewhere.
//
// Verifies: REQ-AUTH-026
func TestPDMCredentialByName(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "pdm", "config.toml"),
		"[pypi.corp]\nurl = \"https://pdm.corp/simple\"\nusername = \"u\"\npassword = \"p\"\n")
	s := pythonRun(t, home, nil, map[string]string{"pyproject.toml": `
[[tool.pdm.source]]
name = "corp"
url = "https://pdm.corp/team/simple"
`, "sub/pyproject.toml": `
[[tool.pdm.source]]
name = "corp"
url = "https://evil.example/simple"
`})
	if got := authOf(s, "https://pdm.corp/team/pypi/a/json"); got != basicHeaderOf("u:p") {
		t.Errorf("re-pointed on the machine's host: %q", got)
	}
	if got := authOf(s, "https://evil.example/pypi/a/json"); got != "" {
		t.Errorf("elsewhere: %q", got)
	}
}

// End to end: a uv index this machine names by UV_INDEX, with its
// UV_INDEX_<NAME>_* credential, answers the package asked of it.
//
// Verifies: REQ-AUTH-026, REQ-SUP-066
func TestUVIndexCredentialEndToEnd(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.Path+" "+r.Header.Get("Authorization"))
		mu.Unlock()
		if r.Header.Get("Authorization") != basicHeaderOf("ci:secret") || r.URL.Path != "/team/pypi/lib/1.0.0/json" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, pypiJSON("certifi"))
	}))
	t.Cleanup(registry.Close)
	pypi := newFeed(t, nil)
	asPublic(t, PyPI, pypi)
	e := environment(map[string]string{
		"UV_DEFAULT_INDEX":       "corp=" + registry.URL + "/team/simple",
		"UV_INDEX_CORP_USERNAME": "ci", "UV_INDEX_CORP_PASSWORD": "secret",
	})
	home := t.TempDir()
	store := auth.Read(home, e)
	d := NewDiscoverer(e, home)
	d.Config().Credentials(store)
	c := NewClient(d.Discover(nil), t.TempDir(), time.Hour, 5*time.Second, store, nil)
	if got := authOf(store, registry.URL+"/team/pypi/lib/1.0.0/json"); got != basicHeaderOf("ci:secret") {
		t.Fatalf("credential %q", got)
	}
	if got := names(c.Dependencies(lang.Target{Ecosystem: PyPI, Package: "lib", Version: "1.0.0"})); !slices.Equal(got, []string{"certifi"}) {
		t.Errorf("got %v, asked %v", got, seen)
	}
}
