package index

import (
	"archive/zip"
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

// uv reads a project's configuration before the user's uv.toml: the repository's
// uv indexes go before the user's, its default index wins over the user's
// default, and uv's variables (UV_INDEX, UV_DEFAULT_INDEX) still come first.
// What other tools of the repository name keeps its place after this machine's.
//
// Verifies: REQ-SUP-066, REQ-SUP-063
func TestUVProjectIndexesComeBeforeTheUsersFiles(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "uv", "uv.toml"), "[[index]]\nurl = \"https://user-extra.corp/simple\"\n\n"+
		"[[index]]\nurl = \"https://user-default.corp/simple\"\ndefault = true\n")
	files := write(t, map[string]string{"pyproject.toml": `
[[tool.pdm.source]]
name = "pdm"
url = "https://pdm.corp/simple"

[[tool.uv.index]]
url = "https://project-extra.corp/simple"

[[tool.uv.index]]
url = "https://project-default.corp/simple"
default = true
`})
	c := discoverOn(home, "linux", map[string]string{"UV_INDEX": "https://env-extra.corp/simple"})
	c.project(files)
	got := strings.Join(sources(c, PyPI), " ")
	want := "https://env-extra.corp/simple https://project-extra.corp/simple? https://project-default.corp/simple? " +
		"https://user-extra.corp/simple https://user-default.corp/simple https://pdm.corp/simple?"
	if got != want {
		t.Errorf("sources\n got %s\nwant %s", got, want)
	}
	if got, want := order(c, PyPI, "x", ""), []string{"https://env-extra.corp/simple", "https://project-extra.corp/simple?",
		"https://user-extra.corp/simple", "https://pdm.corp/simple?", "https://project-default.corp/simple?"}; !slices.Equal(got, want) {
		t.Errorf("asked %v, want %v", got, want)
	}

	c = discoverOn(home, "linux", map[string]string{"UV_DEFAULT_INDEX": "https://env-default.corp/simple"})
	c.project(files)
	if got, want := order(c, PyPI, "x", ""), []string{"https://project-extra.corp/simple?", "https://user-extra.corp/simple",
		"https://pdm.corp/simple?", "https://env-default.corp/simple"}; !slices.Equal(got, want) {
		t.Errorf("UV_DEFAULT_INDEX: asked %v, want %v", got, want)
	}

	// Without a uv.toml of the user's, the repository's indexes simply follow
	// this machine's.
	c = discoverOn(t.TempDir(), "linux", map[string]string{"UV_INDEX": "https://env-extra.corp/simple"})
	c.project(files)
	if got, want := order(c, PyPI, "x", ""), []string{"https://env-extra.corp/simple", "https://pdm.corp/simple?",
		"https://project-extra.corp/simple?", "https://project-default.corp/simple?"}; !slices.Equal(got, want) {
		t.Errorf("no uv.toml: asked %v, want %v", got, want)
	}
}

// PDM asks every source that includes a package (include_packages) and nothing
// else; a package no source includes is asked of the sources that do not exclude
// it (exclude_packages) - and not of PyPI when the pypi source excludes it.
//
// Verifies: REQ-SUP-066
func TestPDMIncludeAndExclude(t *testing.T) {
	c := Discover(write(t, map[string]string{"pyproject.toml": `
[[tool.pdm.source]]
name = "pypi"
url = "https://pypi.org/simple"
exclude_packages = ["internal-*"]

[[tool.pdm.source]]
name = "a"
url = "https://a.corp/simple"
include_packages = ["acme-*", "shared"]

[[tool.pdm.source]]
name = "b"
url = "https://b.corp/simple"
include_packages = ["Shared"]
exclude_packages = ["requests"]

[[tool.pdm.source]]
name = "c"
url = "https://c.corp/simple"
`}), environment(nil), "")
	for packageName, want := range map[string][]string{
		"shared":        {"https://a.corp/simple?", "https://b.corp/simple?"},
		"acme.billing":  {"https://a.corp/simple?"},
		"requests":      {"https://a.corp/simple?", "https://c.corp/simple?", "https://pypi.org/simple"},
		"other":         {"https://a.corp/simple?", "https://b.corp/simple?", "https://c.corp/simple?", "https://pypi.org/simple"},
		"Internal_Tool": {"https://a.corp/simple?", "https://b.corp/simple?", "https://c.corp/simple?"},
	} {
		if got := order(c, PyPI, packageName, ""); !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", packageName, got, want)
		}
	}
}

// Under respect-source-order PDM asks its sources in order: PyPI first unless a
// source named pypi takes its place, the sources after it only for what it lacks.
//
// Verifies: REQ-SUP-066, REQ-SUP-063
func TestPDMRespectSourceOrder(t *testing.T) {
	c := Discover(write(t, map[string]string{"pyproject.toml": `
[tool.pdm.resolution]
respect-source-order = true

[[tool.pdm.source]]
name = "first"
url = "https://first.corp/simple"

[[tool.pdm.source]]
name = "pypi"
url = "https://mirror.corp/simple"

[[tool.pdm.source]]
name = "last"
url = "https://last.corp/simple"
`}), environment(nil), "")
	if got, want := order(c, PyPI, "x", ""), []string{"https://first.corp/simple?", "https://mirror.corp/simple?",
		"https://last.corp/simple?"}; !slices.Equal(got, want) {
		t.Errorf("asked %v, want %v", got, want)
	}
	c = Discover(write(t, map[string]string{"pyproject.toml": "[tool.pdm.resolution]\nrespect-source-order = true\n\n" +
		"[[tool.pdm.source]]\nname = \"corp\"\nurl = \"https://corp.example/simple\"\n"}), environment(nil), "")
	if got, want := order(c, PyPI, "x", ""), []string{"https://pypi.org/simple", "https://corp.example/simple?"}; !slices.Equal(got, want) {
		t.Errorf("without a pypi source: asked %v, want %v", got, want)
	}
}

// A package manager that merges what several indexes have - uv under
// index-strategy unsafe-best-match or with find-links locations, PDM without
// respect-source-order - is noted once, when a package is asked of more than one
// index. UV_INDEX_STRATEGY wins over the repository's setting, which wins over the
// user's uv.toml.
//
// Verifies: REQ-SUP-066, REQ-TRC-017
func TestPythonMergesAreNoted(t *testing.T) {
	pypi := newFeed(t, map[string]string{"/pypi/lib/1.0/json": pypiJSON("certifi")})
	extra := newFeed(t, nil)
	asPublic(t, PyPI, pypi)
	target := lang.Target{Ecosystem: PyPI, Package: "lib", Version: "1.0"}
	run := func(variables map[string]string, files map[string]string, userFile string) []string {
		home := t.TempDir()
		if userFile != "" {
			put(t, filepath.Join(home, ".config", "uv", "uv.toml"), userFile)
		}
		d := NewDiscoverer(environment(variables), home)
		d.Config().Trust([]string{extra.URL + "/simple"})
		return notesOf(t, newClient(t, d.Discover(write(t, files))), target, target)
	}
	index := map[string]string{"UV_INDEX": extra.URL + "/simple"}
	best := map[string]string{"UV_INDEX": extra.URL + "/simple", "UV_INDEX_STRATEGY": "unsafe-best-match"}
	strategy := map[string]string{"pyproject.toml": "[tool.uv]\nindex-strategy = \"unsafe-best-match\"\n"}
	pdm := map[string]string{"pyproject.toml": "[[tool.pdm.source]]\nname = \"x\"\nurl = \"" + extra.URL + "/simple\"\n"}
	pdmOrdered := map[string]string{"pyproject.toml": "[tool.pdm.resolution]\nrespect-source-order = true\n\n" +
		"[[tool.pdm.source]]\nname = \"x\"\nurl = \"" + extra.URL + "/simple\"\n"}
	for _, testCase := range []struct {
		name       string
		variables  map[string]string
		files      map[string]string
		userFile   string
		wantPrefix string
	}{
		{"variable", best, nil, "", "merged: uv's index-strategy unsafe-best-match"},
		{"one index", map[string]string{"UV_INDEX_STRATEGY": "unsafe-best-match"}, nil, "", ""},
		{"first-index", index, nil, "", ""},
		{"repository", index, strategy, "", "merged: uv's index-strategy unsafe-best-match"},
		{"variable over repository", map[string]string{"UV_INDEX": extra.URL + "/simple", "UV_INDEX_STRATEGY": "first-index"}, strategy, "", ""},
		{"user file", index, nil, "index-strategy = \"unsafe-best-match\"\n", "merged: uv's index-strategy unsafe-best-match"},
		{"repository over user file", index, map[string]string{"uv.toml": "index-strategy = \"unsafe-first-match\"\n"},
			"index-strategy = \"unsafe-best-match\"\n", ""},
		{"find-links", map[string]string{"UV_FIND_LINKS": extra.URL + "/simple"}, nil, "", "merged: uv adds the files of its find-links"},
		{"pdm", nil, pdm, "", "merged: PDM merges the versions of all its sources"},
		{"pdm in order", nil, pdmOrdered, "", ""},
	} {
		got := run(testCase.variables, testCase.files, testCase.userFile)
		if testCase.wantPrefix == "" && len(got) != 0 ||
			testCase.wantPrefix != "" && (len(got) != 1 || !strings.HasPrefix(got[0], testCase.wantPrefix) ||
				!strings.HasSuffix(got[0], "depphunter asks the indexes in order and takes the first that has the package")) {
			t.Errorf("%s: notes %q", testCase.name, got)
		}
	}
}

// flatListing lists files the way a find-links page does: relative links, PEP 658
// metadata advertised for one wheel, a pre-release, a source archive without
// metadata.
const flatListing = `<html><body>
<a href="lib-1.0-py3-none-any.whl#sha256=00" data-dist-info-metadata="sha256={{sha}}">lib-1.0-py3-none-any.whl</a>
<a href="lib-1.0.tar.gz">lib-1.0.tar.gz</a>
<a href="lib-2.0rc1-py3-none-any.whl">lib-2.0rc1-py3-none-any.whl</a>
<a href="bare-1.0.tar.gz">bare-1.0.tar.gz</a>
</body></html>`

// A flat index this machine names (UV_FIND_LINKS) is one page read once: a
// release it lists is answered from the metadata its wheel has beside it, one
// without metadata says so and stops there, and a package the page does not list
// is asked of the next index.
//
// Verifies: REQ-SUP-067, REQ-SUP-066
func TestFlatIndexPage(t *testing.T) {
	metadata := "Metadata-Version: 2.1\nName: lib\nVersion: 1.0\nRequires-Dist: certifi\n"
	stub := newSimpleIndex(t, "", map[string]served{
		"/links/": {"text/html", strings.ReplaceAll(flatListing, "{{sha}}", sha(metadata))},
		"/links/lib-1.0-py3-none-any.whl.metadata": {"text/plain", metadata},
	})
	pypi := newFeed(t, map[string]string{"/pypi/requests/2.0/json": pypiJSON("idna")})
	asPublic(t, PyPI, pypi)
	c := newClient(t, Discover(nil, environment(map[string]string{"UV_FIND_LINKS": stub.URL + "/links/"}), t.TempDir()))
	if got, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: "Lib"}); !slices.Equal(got, []string{"certifi"}) || l.Index != stub.URL+"/links" {
		t.Errorf("lib: %v from %s (%s)", got, l.Index, l.Reason)
	}
	if got, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: "requests", Version: "2.0"}); !slices.Equal(got, []string{"idna"}) || l.Index != pypi.URL {
		t.Errorf("requests: %v from %s (%s)", got, l.Index, l.Reason)
	}
	if _, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: "bare", Version: "1.0"}); !strings.Contains(l.Reason, errNoMetadata.Error()) || pypi.askedFor("bare") {
		t.Errorf("bare: %q, PyPI asked %v", l.Reason, pypi.paths())
	}
	pages := 0
	for _, r := range stub.requests() {
		if r == "/links/" {
			pages++
		}
	}
	if pages != 1 {
		t.Errorf("page read %d times: %v", pages, stub.requests())
	}
}

// wheel writes a wheel holding a METADATA file in its .dist-info directory.
func wheel(t *testing.T, name, distInfo, metadata string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	// A METADATA deeper down (a vendored package's) comes first and is not the
	// wheel's.
	for _, entry := range [][2]string{{"vendored/" + distInfo + "/METADATA", "Requires-Dist: wrong\n"},
		{"pkg/__init__.py", ""}, {distInfo + "/METADATA", metadata}} {
		if part, err := w.Create(entry[0]); err == nil {
			part.Write([]byte(entry[1]))
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

// A local directory this machine's uv.toml names as find-links (relative to the
// file) is read from disk: a wheel's METADATA, a .metadata file beside a source
// archive; a release with only a source archive stops with the reason, and a
// package the directory lacks goes on to PyPI.
//
// Verifies: REQ-SUP-067, REQ-SUP-066
func TestLocalFlatIndex(t *testing.T) {
	home := t.TempDir()
	directory := filepath.Join(home, ".config", "uv", "wheels")
	put(t, filepath.Join(home, ".config", "uv", "uv.toml"), "find-links = [\"wheels\"]\n")
	wheel(t, filepath.Join(directory, "corp_lib-1.0-py3-none-any.whl"), "corp_lib-1.0.dist-info", "Name: corp-lib\nRequires-Dist: certifi\n")
	put(t, filepath.Join(directory, "tool-2.0.tar.gz"), "not read")
	put(t, filepath.Join(directory, "tool-2.0.tar.gz.metadata"), "Name: tool\nRequires-Dist: idna\n")
	put(t, filepath.Join(directory, "only-1.0.tar.gz"), "not read")
	pypi := newFeed(t, map[string]string{"/pypi/requests/2.0/json": pypiJSON("urllib3")})
	asPublic(t, PyPI, pypi)
	d := NewDiscoverer(environment(nil), home)
	config := d.Discover(nil)
	if got := sources(config, PyPI); !slices.Equal(got, []string{directory}) {
		t.Fatalf("sources %v", got)
	}
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, environment(nil)))
	for packageName, want := range map[string]string{"corp.lib": "certifi", "tool": "idna", "requests": "urllib3"} {
		version := map[string]string{"corp.lib": "1.0", "tool": "", "requests": "2.0"}[packageName]
		if got, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: packageName, Version: version}); !slices.Equal(got, []string{want}) {
			t.Errorf("%s: %v from %s (%s)", packageName, got, l.Index, l.Reason)
		}
	}
	if _, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: "only", Version: "1.0"}); !strings.Contains(l.Reason, errNoLocalMetadata.Error()) {
		t.Errorf("only: %q", l.Reason)
	}
}

// A flat index the repository names is untrusted like any repository index: a
// page URL is recorded and not read until the user vouches for it, a directory is
// not recorded at all. A package pinned to a flat index is read from it.
//
// Verifies: REQ-SUP-066, REQ-SUP-043
func TestRepositoryFlatIndexes(t *testing.T) {
	metadata := "Name: lib\nRequires-Dist: certifi\n"
	stub := newSimpleIndex(t, "", map[string]served{
		"/links/": {"text/html", strings.ReplaceAll(flatListing, "{{sha}}", sha(metadata))},
		"/links/lib-1.0-py3-none-any.whl.metadata": {"text/plain", metadata},
	})
	pypi := newFeed(t, nil)
	asPublic(t, PyPI, pypi)
	files := write(t, map[string]string{"pyproject.toml": `
[tool.uv.sources]
lib = { index = "page" }

[[tool.uv.index]]
name = "local"
url = "./wheels"
format = "flat"

[[tool.uv.index]]
name = "page"
url = "` + stub.URL + `/links/"
format = "flat"
explicit = true
`})
	config := Discover(files, environment(nil), t.TempDir())
	if got := sources(config, PyPI); !slices.Equal(got, []string{"lib " + stub.URL + "/links?"}) {
		t.Errorf("sources %v", got)
	}
	if _, l := ask(t, newClient(t, config), lang.Target{Ecosystem: PyPI, Package: "lib", Version: "1.0"}); l.Reason != trace.ReasonUntrusted {
		t.Errorf("unvouched: %+v", l)
	}
	if len(stub.requests()) != 0 {
		t.Errorf("asked %v", stub.requests())
	}
	d := NewDiscoverer(environment(nil), t.TempDir())
	d.Config().Trust([]string{stub.URL + "/links/"})
	if got, l := ask(t, newClient(t, d.Discover(files)), lang.Target{Ecosystem: PyPI, Package: "lib", Version: "1.0"}); !slices.Equal(got, []string{"certifi"}) {
		t.Errorf("vouched: %v (%s)", got, l.Reason)
	}
}
