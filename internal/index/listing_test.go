package index

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scope"
)

// publicClient is a client whose ecosystem's public index is url, over cfg (a
// fresh configuration when nil). The public index is replaced before config is read
// by the caller when discovery must see it; see withPublic.
func publicClient(t *testing.T, ecosystem, url string, config *Config) *Client {
	t.Helper()
	withPublic(t, ecosystem, url)
	if config == nil {
		config = New()
	}
	return NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil, nil)
}

// withPublic makes url the public index of ecosystem for the rest of the test.
func withPublic(t *testing.T, ecosystem, url string) {
	t.Helper()
	was := public[ecosystem]
	public[ecosystem] = url
	t.Cleanup(func() { public[ecosystem] = was })
}

// withGitHub points GitHub's API at url for the rest of the test.
func withGitHub(t *testing.T, url string) {
	t.Helper()
	was := githubAPI
	githubAPI = url
	t.Cleanup(func() { githubAPI = was })
}

// recorder is a stub server's log of the requests it was sent, path and query.
type recorder struct {
	mu    sync.Mutex
	asked []string
}

func (r *recorder) add(request *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u := request.URL.Path
	if request.URL.RawQuery != "" {
		u += "?" + request.URL.RawQuery
	}
	r.asked = append(r.asked, u)
}

func (r *recorder) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.asked
	r.asked = nil
	return out
}

// files serves a map of paths to bodies, 404 for anything else.
func files(recorded *recorder, bodies map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded.add(r)
		key := r.URL.Path
		if r.URL.RawQuery != "" {
			key += "?" + r.URL.RawQuery
		}
		body, ok := bodies[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
}

func gzipped(t *testing.T, data string) string {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	z.Write([]byte(data))
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func dependencyNames(dependencies []lang.Target) []string {
	out := []string{}
	for _, d := range dependencies {
		out = append(out, d.Package+" "+d.Version)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------- CPAN

// A distribution at one version is that release on MetaCPAN: by the author the
// repository's cpanfile.snapshot names, else found by the release search; a
// version MetaCPAN has no release of is answered with the latest release's
// dependencies and a note. A range asks for the latest release.
//
// Verifies: REQ-SUP-053, REQ-TRC-017
func TestCPANPinnedRelease(t *testing.T) {
	release := func(dependencies ...string) string {
		var list []string
		for _, m := range dependencies {
			list = append(list, `{"module": "`+m+`", "version": "0", "phase": "runtime", "relationship": "requires"}`)
		}
		return `{"distribution": "Plack", "dependency": [` + strings.Join(list, ",") + `]}`
	}
	recorded := &recorder{}
	search := func(v string) string {
		return "/v1/release/_search?q=distribution%3A%22Plack%22+AND+version%3A%22" + v + "%22&size=1"
	}
	server := files(recorded, map[string]string{
		"/v1/release/MIYAGAWA/Plack-1.0050": release("Try::Tiny"),
		search("1.0047"):                    `{"hits": {"hits": [{"_source": ` + release("HTTP::Message") + `}]}}`,
		search("0.9"):                       `{"hits": {"hits": []}}`,
		"/v1/release/Plack":                 release("Cookie::Baker"),
		"/v1/module/Try::Tiny":              `{"distribution": "Try-Tiny"}`,
		"/v1/module/HTTP::Message":          `{"distribution": "HTTP-Message"}`,
		"/v1/module/Cookie::Baker":          `{"distribution": "Cookie-Baker"}`,
	})
	defer server.Close()
	config := Discover(write(t, map[string]string{"app/cpanfile.snapshot": `# carton snapshot format: version 1.0
DISTRIBUTIONS
  Plack-1.0050
    pathname: M/MI/MIYAGAWA/Plack-1.0050.tar.gz
    provides:
      Plack 1.0050
`}), environment(nil), "")
	c := publicClient(t, CPAN, server.URL, config)
	for _, test := range []struct {
		version string
		want    []string
		asked   []string
		note    bool
	}{
		{"1.0050", []string{"Try-Tiny "}, []string{"/v1/release/MIYAGAWA/Plack-1.0050", "/v1/module/Try::Tiny"}, false},
		{"1.0047", []string{"HTTP-Message "}, []string{search("1.0047"), "/v1/module/HTTP::Message"}, false},
		{"0.9", []string{"Cookie-Baker "}, []string{search("0.9"), "/v1/release/Plack", "/v1/module/Cookie::Baker"}, true},
		{">= 1.0", []string{"Cookie-Baker "}, []string{"/v1/release/Plack"}, false},
	} {
		notes := notesOf(t, c, lang.Target{Ecosystem: CPAN, Package: "Plack", Version: test.version})
		got := dependencyNames(c.Dependencies(lang.Target{Ecosystem: CPAN, Package: "Plack", Version: test.version}))
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: got %v, want %v", test.version, got, test.want)
		}
		if asked := recorded.take(); !reflect.DeepEqual(asked, test.asked) {
			t.Errorf("%s: asked %v, want %v", test.version, asked, test.asked)
		}
		if (len(notes) == 1 && strings.HasPrefix(notes[0], "no-release: MetaCPAN has no release Plack-0.9")) != test.note {
			t.Errorf("%s: notes %v", test.version, notes)
		}
	}
}

// cpanm's mirrors are indexes only under --mirror-only (or --from), in order,
// and CPAN itself only when it is one of them; Carton's mirror is asked before
// CPAN; a directory is a file: mirror.
//
// Verifies: REQ-SUP-053, REQ-SUP-064
func TestCPANMirrorDiscovery(t *testing.T) {
	metacpan := "https://fastapi.metacpan.org"
	for _, test := range []struct {
		variables map[string]string
		want      []string
	}{
		{nil, []string{metacpan}},
		{map[string]string{"PERL_CPANM_OPT": "--mirror https://darkpan.corp/ --mirror-only"}, []string{"https://darkpan.corp"}},
		{map[string]string{"PERL_CPANM_OPT": "--mirror-only --mirror=https://darkpan.corp --mirror https://www.cpan.org"},
			[]string{"https://darkpan.corp", metacpan}},
		{map[string]string{"PERL_CPANM_OPT": "-q --from 'https://pinto.corp:3111'"}, []string{"https://pinto.corp:3111"}},
		{map[string]string{"PERL_CPANM_OPT": "--mirror https://downloads.corp"}, []string{metacpan}},
		{map[string]string{"PERL_CPANM_OPT": "--mirror /srv/minicpan --mirror-only"}, []string{"file:///srv/minicpan"}},
		{map[string]string{"PERL_CARTON_MIRROR": "https://carton.corp/"}, []string{"https://carton.corp", metacpan}},
		{map[string]string{"PERL_CARTON_MIRROR": "https://cpan.metacpan.org/"}, []string{metacpan}},
	} {
		c := discoverOn(t.TempDir(), "linux", test.variables)
		if got := order(c, CPAN, "Plack", ""); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%v: %v, want %v", test.variables, got, test.want)
		}
	}
}

// A CPAN mirror's 02packages (read once, gzipped or not, over HTTP or from a
// directory) says which distributions it has: one it lacks is not found there;
// one it has is answered with the same release's dependencies on MetaCPAN (the
// mirror's author and version, or the version asked for), each module named by
// the distribution the mirror lists; a release MetaCPAN does not describe gets
// no dependencies and a note, and a private distribution is not named to
// MetaCPAN at all.
//
// Verifies: REQ-SUP-053, REQ-TRC-017
func TestCPANMirror(t *testing.T) {
	packages := `File:         02packages.details.txt
Line-Count:   7

Plack::Legacy        0.9  M/MI/MIYAGAWA/Plack-0.9.tar.gz
Corp::Billing        1.2  C/CO/CORP/Corp-Billing-1.2.tar.gz
Corp::Billing::Util  1.2  C/CO/CORP/Corp-Billing-1.2.tar.gz
Plack::Util       1.0050  M/MI/MIYAGAWA/Plack-1.0050.tar.gz
Plack             1.0050  M/MI/MIYAGAWA/Plack-1.0050.tar.gz
HTTP::Message       6.45  O/OA/OALDERS/HTTP-Message-6.45.tar.gz
Carp                1.50  S/SH/SHAY/perl-5.38.0.tar.gz
Secret::Thing        0.1  C/CO/CORP/Secret-Thing-0.1.tar.gz
`
	mirrorRecorder, metadataRecorder := &recorder{}, &recorder{}
	mirror := files(mirrorRecorder, map[string]string{"/modules/02packages.details.txt.gz": gzipped(t, packages)})
	defer mirror.Close()
	release := `{"distribution": "Plack", "dependency": [
		{"module": "HTTP::Message", "version": "5.814", "phase": "runtime", "relationship": "requires"},
		{"module": "Carp", "version": "0", "phase": "runtime", "relationship": "requires"},
		{"module": "Plack::Util", "version": "0", "phase": "runtime", "relationship": "requires"},
		{"module": "Not::Mirrored", "version": "0", "phase": "runtime", "relationship": "requires"},
		{"module": "Test::More", "version": "0", "phase": "test", "relationship": "requires"}]}`
	metadata := files(metadataRecorder, map[string]string{
		"/v1/release/MIYAGAWA/Plack-1.0050": release,
		"/v1/release/MIYAGAWA/Plack-1.0048": release,
	})
	defer metadata.Close()
	withPublic(t, CPAN, metadata.URL)
	config := discoverOn(t.TempDir(), "linux", map[string]string{"PERL_CPANM_OPT": "--mirror " + mirror.URL + " --mirror-only"})
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil, scope.New([]string{"cpan:Secret-*"}))

	want := []string{"HTTP-Message >= 5.814"}
	if got := dependencyNames(c.Dependencies(lang.Target{Ecosystem: CPAN, Package: "Plack"})); !reflect.DeepEqual(got, want) {
		t.Errorf("Plack: %v", got)
	}
	if got := dependencyNames(c.Dependencies(lang.Target{Ecosystem: CPAN, Package: "Plack", Version: "1.0048", Pinned: true})); !reflect.DeepEqual(got, want) {
		t.Errorf("Plack 1.0048: %v", got)
	}
	notes := notesOf(t, c, lang.Target{Ecosystem: CPAN, Package: "Corp-Billing"})
	if len(notes) != 1 || !strings.Contains(notes[0], "C/CO/CORP/Corp-Billing-1.2.tar.gz") {
		t.Errorf("Corp-Billing notes %v", notes)
	}
	for _, name := range []string{"Secret-Thing", "Absent-Dist"} {
		if got := c.Dependencies(lang.Target{Ecosystem: CPAN, Package: name}); len(got) != 0 {
			t.Errorf("%s: %v", name, got)
		}
	}
	if got := mirrorRecorder.take(); !reflect.DeepEqual(got, []string{"/modules/02packages.details.txt.gz"}) {
		t.Errorf("mirror asked %v", got)
	}
	wantMeta := []string{"/v1/release/MIYAGAWA/Plack-1.0050", "/v1/release/MIYAGAWA/Plack-1.0048", "/v1/release/CORP/Corp-Billing-1.2"}
	if got := metadataRecorder.take(); !reflect.DeepEqual(got, wantMeta) {
		t.Errorf("MetaCPAN asked %v", got)
	}

	// A minicpan directory, its 02packages not compressed, as Carton's mirror.
	directory := t.TempDir()
	put(t, filepath.Join(directory, "modules", "02packages.details.txt"), packages)
	config = discoverOn(t.TempDir(), "linux", map[string]string{"PERL_CARTON_MIRROR": directory})
	if got := order(config, CPAN, "Plack", ""); len(got) != 2 || !strings.HasPrefix(got[0], "file://") {
		t.Fatalf("carton mirror %v", got)
	}
	c = NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil, nil)
	if got := dependencyNames(c.Dependencies(lang.Target{Ecosystem: CPAN, Package: "Plack"})); !reflect.DeepEqual(got, want) {
		t.Errorf("file mirror Plack: %v", got)
	}
	// Not on the DarkPAN: asked of MetaCPAN, after it.
	c.Dependencies(lang.Target{Ecosystem: CPAN, Package: "Moo"})
	if got := metadataRecorder.take(); !reflect.DeepEqual(got, []string{"/v1/release/MIYAGAWA/Plack-1.0050", "/v1/release/Moo"}) {
		t.Errorf("MetaCPAN asked %v", got)
	}
}

// ---------------------------------------------------------------- opam

// opamRoot writes an opam root: repos-config, the root config, a switch's
// switch-config, and the copies opam keeps (a directory and an archive).
func opamRoot(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		put(t, filepath.Join(root, filepath.FromSlash(name)), body)
	}
}

// opamArchive writes repo/<name>.tar.gz with opam files below default/packages.
func opamArchive(t *testing.T, file string, opamFiles map[string]string) {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	tarWriter := tar.NewWriter(z)
	names := make([]string, 0, len(opamFiles))
	for n := range opamFiles {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		body := opamFiles[n]
		tarWriter.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tarWriter.Write([]byte(body))
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	put(t, file, b.String())
}

// opam's repositories are asked in the order of priority the current switch
// gives (OPAMSWITCH, else the root config's switch; its switch-config, else the
// root config's list), opam-repository as the public index, which is off when
// the switch does not use it; an opam root without repos-config leaves the
// public default alone.
//
// Verifies: REQ-SUP-054, REQ-SUP-064
func TestOpamMachineRepositories(t *testing.T) {
	pub := "https://raw.githubusercontent.com/ocaml/opam-repository/master"
	home := t.TempDir()
	root := filepath.Join(home, ".opam")
	opamRoot(t, root, map[string]string{
		"repo/repos-config": `opam-version: "2.0"
repositories: [
  "default" {"https://opam.ocaml.org"}
  "corp" {"git+https://git.corp/opam-repo.git"}
  "extra" {"https://opam.corp/repo" ["fingerprint"] 1}
  "ssh" {"git+ssh://git@git.corp/opam-ssh.git"}
]
`,
		"config":                             "opam-version: \"2.0\"\nrepositories: [\"default\" \"extra\"]\nswitch: \"5.1\"\n",
		"5.1/.opam-switch/switch-config":     "opam-version: \"2.0\"\nrepositories: [\"corp\" \"default\"]\n",
		"repo/corp/packages/x/x.1/opam":      "opam-version: \"2.0\"\n",
		"other/.opam-switch/switch-config":   "opam-version: \"2.0\"\nrepositories: \"corp\"\n",
		"nolist/.opam-switch/switch-config":  "opam-version: \"2.0\"\n",
		"ssh/.opam-switch/switch-config":     "repositories: \"ssh\"\n",
		"repo/default/packages/y/y.1/opam":   "opam-version: \"2.0\"\n",
		"repo/extra/not-a-repository/README": "",
	})
	for _, test := range []struct {
		variables map[string]string
		want      []string
		local     map[string]string
	}{
		{nil, []string{"git+https://git.corp/opam-repo.git", pub},
			map[string]string{"git+https://git.corp/opam-repo.git": filepath.Join(root, "repo", "corp"), pub: filepath.Join(root, "repo", "default")}},
		{map[string]string{"OPAMSWITCH": "other"}, []string{"git+https://git.corp/opam-repo.git"}, nil},
		{map[string]string{"OPAMSWITCH": "nolist"}, []string{pub, "https://opam.corp/repo"},
			map[string]string{"https://opam.corp/repo": ""}},
		{map[string]string{"OPAMROOT": t.TempDir()}, []string{pub}, nil},
		{map[string]string{"OPAMSWITCH": "ssh"}, []string{"git+ssh://git.corp/opam-ssh.git"}, nil},
	} {
		c := discoverOn(home, "linux", test.variables)
		if got := order(c, Opam, "lwt", ""); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%v: %v, want %v", test.variables, got, test.want)
		}
		for u, want := range test.local {
			if got := c.localCopy(Opam, u); got != want {
				t.Errorf("%v: copy of %s is %q, want %q", test.variables, u, got, want)
			}
		}
	}
	// opam 2.1 and later keep a repository as an archive.
	os.RemoveAll(filepath.Join(root, "repo", "default"))
	opamArchive(t, filepath.Join(root, "repo", "default.tar.gz"), nil)
	if got := discoverOn(home, "linux", nil).localCopy(Opam, pub); got != filepath.Join(root, "repo", "default.tar.gz") {
		t.Errorf("archive copy %q", got)
	}
	// Windows: %LOCALAPPDATA%\opam.
	local := t.TempDir()
	opamRoot(t, filepath.Join(local, "opam"), map[string]string{"repo/repos-config": "repositories: \"corp\" {\"https://opam.corp/win\"}\n"})
	if got := order(discoverOn(t.TempDir(), "windows", map[string]string{"LOCALAPPDATA": local}), Opam, "lwt", ""); !reflect.DeepEqual(got, []string{"https://opam.corp/win"}) {
		t.Errorf("windows %v", got)
	}
}

// A dune-workspace's repositories are the repository's own (never asked unless
// vouched for): in its lock_dir's order, `:standard` standing for overlay and
// upstream, opam-repository switched off when the list leaves it out; beside it
// when no lock_dir lists any.
//
// Verifies: REQ-SUP-054
func TestDuneWorkspaceRepositories(t *testing.T) {
	pub := "https://raw.githubusercontent.com/ocaml/opam-repository/master"
	corp := "git+https://git.corp/opam-repo.git"
	stanza := "(lang dune 3.16)\n(repository\n (name corp)\n (url " + corp + "))\n"
	for _, test := range []struct {
		lock string
		want []string
	}{
		{"", []string{corp + "?", pub}},
		{"(lock_dir (path dune.lock) (repositories corp :standard))", []string{corp + "?", opamOverlays + "?", pub}},
		{"(lock_dir (repositories corp))", []string{corp + "?"}},
		{"(lock_dir (repositories upstream))", []string{pub}},
	} {
		c := Discover(write(t, map[string]string{"dune-workspace": stanza + test.lock + "\n"}), environment(nil), "")
		if got := order(c, Opam, "lwt", ""); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%q: %v, want %v", test.lock, got, test.want)
		}
	}
}

// A range is answered by the newest version it admits (opam's order: 1.10 after
// 1.9, 5.10.0 after 5.9.1), listed from the copies opam keeps - a directory, an
// archive read once - with the first repository that has the package deciding;
// nothing goes over the network.
//
// Verifies: REQ-SUP-054
func TestOpamListingFromCopies(t *testing.T) {
	recorded := &recorder{}
	server := files(recorded, nil)
	defer server.Close()
	withPublic(t, Opam, server.URL)
	home := t.TempDir()
	root := filepath.Join(home, ".opam")
	description := func(dependency string) string { return "opam-version: \"2.0\"\ndepends: [\"" + dependency + "\"]\n" }
	opamRoot(t, root, map[string]string{
		"repo/repos-config":                          `repositories: ["corp" {"git+https://git.corp/opam-repo.git"} "default" {"https://opam.ocaml.org"}]`,
		"repo/corp/packages/corpkg/corpkg.1.0/opam":  description("a"),
		"repo/corp/packages/corpkg/corpkg.1.2/opam":  description("b"),
		"repo/corp/packages/corpkg/corpkg.1.10/opam": description("c"),
	})
	opamArchive(t, filepath.Join(root, "repo", "default.tar.gz"), map[string]string{
		"default/packages/lwt/lwt.5.9.1/opam":      description("old"),
		"default/packages/lwt/lwt.5.10.0/opam":     description("new"),
		"default/packages/lwt/lwt.6.0.0~a1/opam":   description("alpha"),
		"default/packages/corpkg/corpkg.9.0/opam":  description("shadowed"),
		"default/packages/lwt/lwt.5.10.0/opam.bak": "",
	})
	c := NewClient(discoverOn(home, "linux", nil), t.TempDir(), time.Hour, 5*time.Second, nil, nil)
	for _, test := range []struct {
		packageName, version string
		want                 []string
	}{
		{"corpkg", "< 1.10", []string{"b "}},
		{"corpkg", "", []string{"c "}},
		{"corpkg", "1.0", []string{"a "}},
		{"lwt", ">= 5.6 & < 6", []string{"new "}},
		{"lwt", "", []string{"alpha "}},
		{"lwt", "5.9.1", []string{"old "}},
	} {
		if got := dependencyNames(c.Dependencies(lang.Target{Ecosystem: Opam, Package: test.packageName, Version: test.version})); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s %q: %v, want %v", test.packageName, test.version, got, test.want)
		}
	}
	if got := c.Dependencies(lang.Target{Ecosystem: Opam, Package: "lwt", Version: "> 7"}); len(got) != 0 {
		t.Errorf("no version admitted: %v", got)
	}
	if got := recorded.take(); len(got) != 0 {
		t.Errorf("asked %v", got)
	}
}

// Without a copy, opam-repository's versions are listed through GitHub's
// contents API (once per package) and the chosen description read as a file; a
// repository only git serves, with no copy, passes the question on with a note;
// an HTTP repository without a copy is not asked about a range.
//
// Verifies: REQ-SUP-054, REQ-TRC-017
func TestOpamListingFromGitHub(t *testing.T) {
	recorded := &recorder{}
	server := files(recorded, map[string]string{
		"/repos/ocaml/opam-repository/contents/packages/fmt?ref=master": `[{"name": "fmt.0.8.9", "type": "dir"},
			{"name": "fmt.0.9.0", "type": "dir"}, {"name": "fmt.0.10.0", "type": "dir"}]`,
		"/packages/fmt/fmt.0.10.0/opam": "depends: [\"ten\"]\n",
		"/packages/fmt/fmt.0.9.0/opam":  "depends: [\"nine\"]\n",
	})
	defer server.Close()
	withPublic(t, Opam, server.URL)
	withGitHub(t, server.URL)
	home := t.TempDir()
	opamRoot(t, filepath.Join(home, ".opam"), map[string]string{
		"repo/repos-config": `repositories: ["corp" {"git+https://git.corp/opam-repo.git"} "web" {"https://opam.corp"} "default" {"https://opam.ocaml.org"}]`,
	})
	config := discoverOn(home, "linux", nil)
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil, nil)
	if got := dependencyNames(c.Dependencies(lang.Target{Ecosystem: Opam, Package: "fmt", Version: ">= 0.9"})); len(got) != 0 {
		t.Errorf("the HTTP repository without a listing answered %v", got)
	}
	config.sources[Opam] = append(config.sources[Opam][:1], config.sources[Opam][2:]...) // without "web"
	notes := notesOf(t, c, lang.Target{Ecosystem: Opam, Package: "fmt", Version: ">= 0.9.0"})
	if len(notes) != 1 || !strings.HasPrefix(notes[0], "no-copy: opam repository git+https://git.corp/opam-repo.git") {
		t.Errorf("notes %v", notes)
	}
	if got := dependencyNames(c.Dependencies(lang.Target{Ecosystem: Opam, Package: "fmt", Version: "< 0.10"})); !reflect.DeepEqual(got, []string{"nine "}) {
		t.Errorf("fmt < 0.10: %v", got)
	}
	want := []string{"/repos/ocaml/opam-repository/contents/packages/fmt?ref=master", "/packages/fmt/fmt.0.10.0/opam", "/packages/fmt/fmt.0.9.0/opam"}
	if got := recorded.take(); !reflect.DeepEqual(got, want) {
		t.Errorf("asked %v", got)
	}
}

// ---------------------------------------------------------------- Alire

// alireCheckout writes an index checkout: index.toml and release manifests.
func alireCheckout(t *testing.T, directory string, releases map[string]string) {
	t.Helper()
	put(t, filepath.Join(directory, "index.toml"), "version = \"1.4.0\"\n")
	for file, dependency := range releases {
		crate := strings.SplitN(file, "-", 2)[0]
		put(t, filepath.Join(directory, crate[:2], crate, file), "[[depends-on]]\n"+dependency+" = \"*\"\n")
	}
}

// alr's indexes are asked by priority (lower first), the community index as the
// public index, off when it is not configured; each with the checkout alr keeps
// (indexes/<name>/repo), or the directory a file: index names. A constraint is
// answered by the newest release it admits, listed from the checkout, a release
// before a pre-release; a commit is not asked about.
//
// Verifies: REQ-SUP-061, REQ-SUP-064
func TestAlireIndexes(t *testing.T) {
	recorded := &recorder{}
	server := files(recorded, nil)
	defer server.Close()
	withPublic(t, Alire, server.URL)
	home := t.TempDir()
	settings := filepath.Join(home, ".config", "alire")
	corp := "git+https://git.corp/alire-index.git"
	put(t, filepath.Join(settings, "indexes", "community", "index.toml"),
		"name = \"community\"\npriority = 2\nurl = \"git+https://github.com/alire-project/alire-index.git#stable-1.4.0\"\n")
	alireCheckout(t, filepath.Join(settings, "indexes", "community", "repo", "index"), map[string]string{
		"aws-25.1.0.toml": "one", "aws-25.2.0.toml": "two", "aws-26.0.0-rc1.toml": "rc", "aws-external.toml": "ext",
	})
	put(t, filepath.Join(settings, "indexes", "corp", "index.toml"), "name = \"corp\"\npriority = 1\nurl = \""+corp+"\"\n")
	alireCheckout(t, filepath.Join(settings, "indexes", "corp", "repo", "index"), map[string]string{
		"corpcrate-1.0.0.toml": "a", "corpcrate-1.4.2.toml": "b", "corpcrate-2.0.0.toml": "c",
	})
	directory := t.TempDir()
	alireCheckout(t, directory, map[string]string{"localcrate-0.1.0.toml": "l"})
	put(t, filepath.Join(settings, "indexes", "mine", "index.toml"),
		"name = \"mine\"\npriority = 3\nurl = \"file:"+filepath.ToSlash(directory)+"\"\n")

	config := discoverOn(home, "linux", nil)
	mine := "file:" + filepath.ToSlash(directory)
	if got := order(config, Alire, "aws", ""); !reflect.DeepEqual(got, []string{corp, server.URL, mine}) {
		t.Errorf("order %v", got)
	}
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil, nil)
	for _, test := range []struct {
		crate, version string
		want           []string
	}{
		{"corpcrate", "^1.0", []string{"b *"}},
		{"corpcrate", ">=1.0 & <1.4.2 | =2.0.0", []string{"c *"}},
		{"aws", "~25.1", []string{"one *"}},
		{"aws", "*", []string{"two *"}},
		{"aws", ">25.2", []string{"rc *"}},
		{"aws", "25.1.0", []string{"one *"}},
		{"localcrate", "", []string{"l *"}},
		{"aws", "73d99ae1ff2f5210dc41c2ea7afebe600f9e9916", []string{}},
		{"aws", "^27", []string{}},
	} {
		if got := dependencyNames(c.Dependencies(lang.Target{Ecosystem: Alire, Package: test.crate, Version: test.version})); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s %q: %v, want %v", test.crate, test.version, got, test.want)
		}
	}
	if got := recorded.take(); len(got) != 0 {
		t.Errorf("asked %v", got)
	}
	// Without the community index, the public one is off; ALIRE_SETTINGS_DIR moves
	// the settings.
	os.RemoveAll(filepath.Join(settings, "indexes", "community"))
	if got := order(discoverOn(home, "linux", nil), Alire, "aws", ""); !reflect.DeepEqual(got, []string{corp, mine}) {
		t.Errorf("without community %v", got)
	}
	if got := order(discoverOn(home, "linux", map[string]string{"ALIRE_SETTINGS_DIR": t.TempDir()}), Alire, "aws", ""); !reflect.DeepEqual(got, []string{server.URL}) {
		t.Errorf("empty settings %v", got)
	}
	// Windows: %USERPROFILE%\.config\alire, whatever XDG_CONFIG_HOME says.
	if got := order(discoverOn(home, "windows", map[string]string{"XDG_CONFIG_HOME": t.TempDir()}), Alire, "aws", ""); !reflect.DeepEqual(got, []string{corp, mine}) {
		t.Errorf("windows %v", got)
	}
}

// Without a checkout, the community index's releases of a crate are listed
// through GitHub's contents API on its branch, and the chosen manifest read as a
// file.
//
// Verifies: REQ-SUP-061
func TestAlireListingFromGitHub(t *testing.T) {
	recorded := &recorder{}
	server := files(recorded, map[string]string{
		"/repos/alire-project/alire-index/contents/index/xm/xmlada?ref=stable-1.4.0": `[{"name": "xmlada-23.0.0.toml"},
			{"name": "xmlada-24.0.0.toml"}, {"name": "xmlada-external.toml"}, {"name": "xmlada-25.0.0-rc.toml"}]`,
		"/index/xm/xmlada/xmlada-24.0.0.toml": "[[depends-on]]\ngnat = \">=13\"\n",
	})
	defer server.Close()
	withGitHub(t, server.URL)
	c := publicClient(t, Alire, server.URL, nil)
	if got := dependencyNames(c.Dependencies(lang.Target{Ecosystem: Alire, Package: "xmlada", Version: "^23 | ^24"})); !reflect.DeepEqual(got, []string{"gnat >=13"}) {
		t.Errorf("got %v", got)
	}
	want := []string{"/repos/alire-project/alire-index/contents/index/xm/xmlada?ref=stable-1.4.0", "/index/xm/xmlada/xmlada-24.0.0.toml"}
	if got := recorded.take(); !reflect.DeepEqual(got, want) {
		t.Errorf("asked %v", got)
	}
}
