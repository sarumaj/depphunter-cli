package index

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// UUIDs of the depot fixture's packages. Corp's registry lists a JSON of its own,
// another package than General's.
const (
	uuidGeneralJSON = "682c06a0-de6a-54ab-a142-c8b1cf79cde6"
	uuidCorpJSON    = "bbbbbbbb-0000-4000-8000-000000000002"
	uuidCorpBilling = "aaaaaaaa-0000-4000-8000-000000000001"
	uuidParsers     = "69de0a69-1ddd-5017-9359-2bf0b02dc9f0"
	uuidTables      = "bd369af6-aec1-5ad0-b16a-f7cc5008161c"
	corpRegistry    = "https://git.corp.example/julia/CorpRegistry.git"
)

// writeTarGzip writes a gzipped tar archive of files, named as a Pkg server's registry
// archives name them ("./Registry.toml").
func writeTarGzip(t *testing.T, name string, files map[string]string) {
	t.Helper()
	var buffer bytes.Buffer
	z := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(z)
	for _, p := range []string{"./", "./C/", "./C/CorpBilling/", "./J/", "./J/JSON/"} {
		tarWriter.WriteHeader(&tar.Header{Name: p, Typeflag: tar.TypeDir, Mode: 0o755})
	}
	for p, body := range files {
		if err := tarWriter.WriteHeader(&tar.Header{Name: "./" + p, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		tarWriter.Write([]byte(body))
	}
	tarWriter.Close()
	z.Close()
	put(t, name, buffer.String())
}

// juliaDepots makes a home whose ~/.julia holds General (a git checkout) and Acme
// (a registry on GitHub, checked out), and a depot ~/corp-depot holding Corp, a
// registry on a private host that a Pkg server served as an archive.
func juliaDepots(t *testing.T) string {
	home := t.TempDir()
	general := filepath.Join(home, ".julia", "registries", "General")
	put(t, filepath.Join(general, "Registry.toml"), `name = "General"
uuid = "`+juliaGeneral+`"
repo = "https://github.com/JuliaRegistries/General.git"

[packages]
`+uuidGeneralJSON+` = { name = "JSON", path = "J/JSON" }
`)
	put(t, filepath.Join(general, "J", "JSON", "Versions.toml"), "[\"0.21.4\"]\ngit-tree-sha1 = \"a\"\n")
	put(t, filepath.Join(general, "J", "JSON", "Deps.toml"), "[0]\nParsers = \""+uuidParsers+"\"\nMmap = \"a63ad114-7e13-5084-954f-fe012c677804\"\n")
	put(t, filepath.Join(general, "J", "JSON", "Compat.toml"), "[0]\nParsers = \"2\"\njulia = \"1\"\n")
	acme := filepath.Join(home, ".julia", "registries", "Acme")
	put(t, filepath.Join(acme, "Registry.toml"), `name = "Acme"
uuid = "44444444-0000-4000-8000-000000000004"
repo = "https://github.com/acme/AcmeRegistry.git"
[packages]
11111111-1111-1111-1111-111111111111 = { name = "AcmeBilling", path = "A/AcmeBilling" }
`)
	put(t, filepath.Join(acme, "A", "AcmeBilling", "Versions.toml"), "[\"3.0.0\"]\ngit-tree-sha1 = \"a\"\n")
	put(t, filepath.Join(acme, "A", "AcmeBilling", "Deps.toml"), "[3]\nJSON = \""+uuidGeneralJSON+"\"\n")

	corp := filepath.Join(home, "corp-depot", "registries")
	put(t, filepath.Join(corp, "Corp.toml"), "git-tree-sha1 = \"0123abcd\"\nuuid = \"33333333-0000-4000-8000-000000000003\"\npath = \"Corp.tar.gz\"\n")
	writeTarGzip(t, filepath.Join(corp, "Corp.tar.gz"), map[string]string{
		"Registry.toml": `name = "Corp"
uuid = "33333333-0000-4000-8000-000000000003"
repo = "` + corpRegistry + `"

[packages]
` + uuidCorpBilling + ` = { name = "CorpBilling", path = "C/CorpBilling" }
` + uuidCorpJSON + ` = { name = "JSON", path = "J/JSON" }
`,
		"C/CorpBilling/Package.toml": "name = \"CorpBilling\"\n",
		"C/CorpBilling/Versions.toml": `["1.1.0"]
git-tree-sha1 = "a"
["1.2.0"]
git-tree-sha1 = "b"
["1.2.5"]
git-tree-sha1 = "c"
["1.3.0"]
git-tree-sha1 = "d"
yanked = true
["2.0.0"]
git-tree-sha1 = "e"
`,
		"C/CorpBilling/Deps.toml": `[1]
Dates = "ade2ca70-3891-5945-98fb-dc099432e06a"
["1.2 - 1.2.4"]
Parsers = "` + uuidParsers + `"
["1.2.5 - 1"]
JSON = "` + uuidGeneralJSON + `"
[2]
Tables = "` + uuidTables + `"
`,
		"C/CorpBilling/Compat.toml": `["1.2.5 - 1"]
JSON = "0.21"
`,
		"J/JSON/Versions.toml": "[\"9.0.0\"]\ngit-tree-sha1 = \"f\"\n",
		"J/JSON/Deps.toml":     "[9]\nTables = \"" + uuidTables + "\"\n",
	})
	return home
}

// offline is a transport that answers nothing and records what was asked.
type offline struct {
	mu    sync.Mutex
	asked []string
}

func (o *offline) RoundTrip(r *http.Request) (*http.Response, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.asked = append(o.asked, r.URL.String())
	return nil, errors.New("offline")
}

// The registries installed in the depots JULIA_DEPOT_PATH lists (an empty entry
// being ~/.julia) are read from their copies - a checkout, or the archive a Pkg
// server served, for a registry on any host - without a request: General's copy
// in place of its files on GitHub, a GitHub-hosted registry's copy in place of
// raw.githubusercontent.com. A package is found by its UUID: Corp's JSON is
// another package than General's, and a target without a UUID goes by name. A
// [compat] range picks the newest release it admits that is not yanked.
//
// Verifies: REQ-SUP-055, REQ-JULIA-010
func TestJuliaDepotRegistries(t *testing.T) {
	home := juliaDepots(t)
	variables := map[string]string{"JULIA_DEPOT_PATH": "~/corp-depot:"}
	config := Discover(nil, environment(variables), home)
	general := public[Julia]
	for _, testCase := range []struct {
		packageName, uuid string
		want              []string
	}{
		{"CorpBilling", uuidCorpBilling, []string{corpRegistry}},
		{"JSON", uuidGeneralJSON, []string{general}},
		{"JSON", strings.ToUpper(uuidCorpJSON), []string{corpRegistry}},
		{"JSON", "", []string{corpRegistry}},
		{"AcmeBilling", "11111111-1111-1111-1111-111111111111", []string{"https://raw.githubusercontent.com/acme/AcmeRegistry/HEAD"}},
		{"CorpBilling", "99999999-0000-4000-8000-000000000009", []string{general}},
	} {
		if got := order(config, Julia, testCase.packageName, testCase.uuid); !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("%s %s: asked of %v, want %v", testCase.packageName, testCase.uuid, got, testCase.want)
		}
	}

	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, environment(variables)))
	net := &offline{}
	c.http = &http.Client{Transport: net}
	std := func(name string) lang.Target { return lang.Target{Ecosystem: "julia-std", Package: name} }
	for _, testCase := range []struct {
		target lang.Target
		want   []lang.Target
	}{
		// "1.2" admits 1.2.0 up to 2: 1.3.0 is yanked, so 1.2.5.
		{lang.Target{Ecosystem: Julia, Package: "CorpBilling", Version: "1.2", Registry: uuidCorpBilling}, []lang.Target{
			std("Dates"), {Ecosystem: Julia, Package: "JSON", Version: "0.21", Registry: uuidGeneralJSON}}},
		{lang.Target{Ecosystem: Julia, Package: "CorpBilling", Version: "1.2.0", Pinned: true, Registry: uuidCorpBilling}, []lang.Target{
			std("Dates"), {Ecosystem: Julia, Package: "Parsers", Registry: uuidParsers}}},
		{lang.Target{Ecosystem: Julia, Package: "JSON", Version: "0.21.4", Pinned: true, Registry: uuidGeneralJSON}, []lang.Target{
			std("Mmap"), {Ecosystem: Julia, Package: "Parsers", Version: "2", Registry: uuidParsers}}},
		{lang.Target{Ecosystem: Julia, Package: "JSON", Registry: uuidCorpJSON}, []lang.Target{
			{Ecosystem: Julia, Package: "Tables", Registry: uuidTables}}},
		{lang.Target{Ecosystem: Julia, Package: "AcmeBilling", Registry: "11111111-1111-1111-1111-111111111111"}, []lang.Target{
			{Ecosystem: Julia, Package: "JSON", Registry: uuidGeneralJSON}}},
	} {
		if got := c.Dependencies(testCase.target); !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("%s %s:\n got %+v\nwant %+v", testCase.target.Package, testCase.target.Version, got, testCase.want)
		}
	}
	// A JSON of a UUID no registry lists is General's question, and General does
	// not have it: its JSON is another package.
	if got := c.Dependencies(lang.Target{Ecosystem: Julia, Package: "JSON", Version: "0.21.4", Pinned: true,
		Registry: "99999999-0000-4000-8000-000000000009"}); got != nil {
		t.Errorf("another JSON answered for: %+v", got)
	}
	if len(net.asked) != 0 {
		t.Errorf("requests made: %v", net.asked)
	}
	// The archive was read once: gone now, it still answers.
	os.Remove(filepath.Join(home, "corp-depot", "registries", "Corp.tar.gz"))
	got := c.Dependencies(lang.Target{Ecosystem: Julia, Package: "CorpBilling", Version: "2.0.0", Pinned: true, Registry: uuidCorpBilling})
	if want := []lang.Target{{Ecosystem: Julia, Package: "Tables", Registry: uuidTables}}; !reflect.DeepEqual(got, want) {
		t.Errorf("2.0.0: got %+v, want %+v", got, want)
	}
}

// With registries installed and General not among them, Pkg asks General about
// nothing: a package no installed registry lists is asked of none. With no
// registry installed at all, General's files on GitHub are asked.
//
// Verifies: REQ-SUP-055
func TestJuliaDepotWithoutGeneral(t *testing.T) {
	home := juliaDepots(t)
	config := Discover(nil, environment(map[string]string{"JULIA_DEPOT_PATH": "~/corp-depot"}), home)
	if got := order(config, Julia, "JSON", uuidGeneralJSON); got != nil {
		t.Errorf("General's JSON asked of %v", got)
	}
	if got := order(config, Julia, "CorpBilling", uuidCorpBilling); !reflect.DeepEqual(got, []string{corpRegistry}) {
		t.Errorf("CorpBilling asked of %v", got)
	}
	config = Discover(nil, environment(map[string]string{"JULIA_DEPOT_PATH": "~/nothing-here"}), home)
	if got := order(config, Julia, "JSON", uuidGeneralJSON); !reflect.DeepEqual(got, []string{public[Julia]}) {
		t.Errorf("no registries: asked of %v", got)
	}
}

// A registry only git serves, with no copy on this machine, is not read: the
// report says so, and the package is asked of the next registry.
//
// Verifies: REQ-SUP-055, REQ-TRC-017
func TestJuliaRegistryWithoutCopyIsNoted(t *testing.T) {
	config := New()
	config.Add(Julia, Source{URL: "ssh://git.corp.example/julia/Registry.git", Scope: "CorpBilling", Trusted: true})
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, auth.Read(t.TempDir(), nil))
	got := notesOf(t, c, lang.Target{Ecosystem: Julia, Package: "CorpBilling", Version: "1.0.0", Pinned: true})
	if len(got) != 1 || !strings.HasPrefix(got[0], trace.NoteNoCopy+": Julia registry ssh://git.corp.example/julia/Registry.git") {
		t.Errorf("notes: %q", got)
	}
}

// General served by a Pkg server is General.toml naming General.tar.gz: it is
// the public index, read from the archive; the archive is not opened before a
// question.
//
// Verifies: REQ-SUP-055
func TestJuliaGeneralArchive(t *testing.T) {
	home := t.TempDir()
	directory := filepath.Join(home, ".julia", "registries")
	put(t, filepath.Join(directory, "General.toml"), "uuid = \""+juliaGeneral+"\"\npath = \"General.tar.gz\"\n")
	put(t, filepath.Join(directory, "General.tar.gz"), "not opened by discovery")
	config := Discover(nil, environment(nil), home)
	writeTarGzip(t, filepath.Join(directory, "General.tar.gz"), map[string]string{
		"Registry.toml":        "name = \"General\"\nuuid = \"" + juliaGeneral + "\"\n[packages]\n" + uuidGeneralJSON + " = { name = \"JSON\", path = \"J/JSON\" }\n",
		"J/JSON/Versions.toml": "[\"0.21.4\"]\ngit-tree-sha1 = \"a\"\n",
		"J/JSON/Deps.toml":     "[0]\nParsers = \"" + uuidParsers + "\"\n",
	})
	if got := order(config, Julia, "JSON", uuidGeneralJSON); !reflect.DeepEqual(got, []string{public[Julia]}) {
		t.Errorf("asked of %v", got)
	}
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, nil))
	net := &offline{}
	c.http = &http.Client{Transport: net}
	got := c.Dependencies(lang.Target{Ecosystem: Julia, Package: "JSON", Registry: uuidGeneralJSON})
	if want := []lang.Target{{Ecosystem: Julia, Package: "Parsers", Registry: uuidParsers}}; !reflect.DeepEqual(got, want) || len(net.asked) != 0 {
		t.Errorf("got %+v (asked %v)", got, net.asked)
	}
}

// A Registry.toml path that climbs out of the registry is left out.
//
// Verifies: REQ-SUP-055
func TestJuliaRegistryPaths(t *testing.T) {
	p := parseJuliaPaths([]byte(`[packages]
` + uuidCorpBilling + ` = { name = "CorpBilling", path = "C/CorpBilling/" }
` + uuidCorpJSON + ` = { name = "JSON", path = "../../etc" }
` + uuidTables + ` = { name = "Tables", path = "/abs/Tables" }
`))
	if got := p.directory("CorpBilling", strings.ToUpper(uuidCorpBilling)); got != "C/CorpBilling" {
		t.Errorf("CorpBilling: %q", got)
	}
	if p.directory("JSON", "") != "" || p.directory("Tables", uuidTables) != "" {
		t.Errorf("paths outside the registry kept: %+v", p)
	}
}
