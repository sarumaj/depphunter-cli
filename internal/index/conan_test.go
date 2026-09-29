package index

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scope"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// cSpell: ignore conanrc conancenter libcurl libnghttp nghttp orev zrev nrev revacme

// conanOrder lists the remotes a Conan package is asked of, in order, with "?"
// after one that is not fetched from.
func conanOrder(c *Config, t lang.Target) []string {
	var out []string
	for _, k := range c.candidatesFor(t) {
		u := k.url
		if !k.known {
			u += "?"
		}
		out = append(out, u)
	}
	return out
}

func conanTarget(name, version, qualifier string) lang.Target {
	return lang.Target{Ecosystem: Conan, Package: name, Version: version, Registry: qualifier}
}

// Conan's remotes.json (in CONAN_HOME, else ~/.conan2) lists the remotes Conan
// asks in order, ConanCenter only when it is one of them; a disabled remote and
// a local-recipes-index are not asked, and a remote's allowed_packages limit it
// to the references they match. A home without remotes.json has ConanCenter.
//
// Verifies: REQ-SUP-076, REQ-SUP-064
func TestConanRemotes(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".conan2", "remotes.json"), `{"remotes": [
  {"name": "acme", "url": "https://conan.acme.test/artifactory/api/conan/conan-local/", "verify_ssl": true,
   "allowed_packages": ["acme-*", "openssl/3.*@"]},
  {"name": "mirror", "url": "https://mirror.acme.test", "verify_ssl": true, "allowed_packages": ["!openssl/*"]},
  {"name": "old", "url": "https://old.acme.test", "verify_ssl": true, "disabled": true},
  {"name": "recipes", "url": "/home/me/recipes", "verify_ssl": true, "remote_type": "local-recipes-index"},
  {"name": "conancenter", "url": "https://center2.conan.io", "verify_ssl": true}]}`)
	config := NewDiscoverer(environment(nil), home).Discover(nil)
	acme, mirror, center := "https://conan.acme.test/artifactory/api/conan/conan-local", "https://mirror.acme.test", "https://center2.conan.io"
	for _, testCase := range []struct {
		target lang.Target
		want   []string
	}{
		{conanTarget("acme-log", "1.0", ""), []string{acme, mirror, center}},
		{conanTarget("acme-log", "[>=1 <2]", ""), []string{acme, mirror, center}},
		{conanTarget("openssl", "3.2.0", ""), []string{acme, center}},
		// "@" at a pattern's end admits no user and channel; "!" negates.
		{conanTarget("openssl", "3.2.0", "@corp/stable"), []string{center}},
		{conanTarget("openssl", "1.1.1w", ""), []string{center}},
		// A range is matched as Conan searches for it: name/*.
		{conanTarget("openssl", "[>=3 <4]", ""), []string{center}},
		{conanTarget("zlib", "1.3.1", ""), []string{mirror, center}},
		{conanTarget("zlib", "", ""), []string{mirror, center}},
	} {
		if got := conanOrder(config, testCase.target); !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("%s/%s: %v, want %v", testCase.target.Package, testCase.target.Version, got, testCase.want)
		}
	}
	if index, known := config.For(Conan, "acme-log"); index != acme || !known {
		t.Errorf("For: %s %v", index, known)
	}
	elsewhere := t.TempDir()
	put(t, filepath.Join(elsewhere, "remotes.json"), `{"remotes": [{"name": "corp", "url": "https://corp.test/conan", "verify_ssl": true}]}`)
	config = NewDiscoverer(environment(map[string]string{"CONAN_HOME": elsewhere}), home).Discover(nil)
	if got := conanOrder(config, conanTarget("zlib", "1.3.1", "")); !reflect.DeepEqual(got, []string{"https://corp.test/conan"}) {
		t.Errorf("CONAN_HOME: %v", got)
	}
	config = NewDiscoverer(environment(nil), t.TempDir()).Discover(nil)
	if got := conanOrder(config, conanTarget("zlib", "1.3.1", "")); !reflect.DeepEqual(got, []string{center}) {
		t.Errorf("no remotes.json: %v", got)
	}
	for remote, want := range map[string]bool{center: true, "https://center.conan.io/": true, acme: false} {
		if got := config.Public(Conan, remote); got != want {
			t.Errorf("Public(%s) = %v", remote, got)
		}
	}
}

// A .conanrc (from a conanfile's directory up to the repository's root) naming a
// Conan home makes that home's remotes the repository's: not asked unless the
// user vouches for them or the user's own remotes.json lists the same remote,
// however the machine's credentials name it. The machine's remotes are asked
// first.
//
// Verifies: REQ-SUP-076, REQ-SUP-043
func TestAConanrcHomeIsTheRepositorys(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".conan2", "remotes.json"), `{"remotes": [
  {"name": "acme", "url": "https://conan.acme.test", "verify_ssl": true}]}`)
	files := write(t, map[string]string{
		".conanrc":              "# the team's home\nconan_home=./.conan-home\n",
		"src/app/conanfile.txt": "[requires]\nzlib/1.3.1\n",
		".conan-home/remotes.json": `{"remotes": [{"name": "evil", "url": "https://evil.test/conan", "verify_ssl": true},
  {"name": "acme", "url": "https://conan.acme.test", "verify_ssl": true}]}`,
	})
	variables := map[string]string{"CONAN_LOGIN_USERNAME": "mona", "CONAN_PASSWORD": "secret"}
	discoverer := NewDiscoverer(environment(variables), home)
	discoverer.Config().Credentials(auth.Read(home, environment(variables)))
	config := discoverer.Discover(files)
	want := []string{"https://conan.acme.test", "https://evil.test/conan?"}
	if got := conanOrder(config, conanTarget("zlib", "1.3.1", "")); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	config.Trust([]string{"https://evil.test/conan"})
	if got := conanOrder(config, conanTarget("zlib", "1.3.1", "")); !reflect.DeepEqual(got, []string{"https://conan.acme.test", "https://evil.test/conan"}) {
		t.Errorf("vouched: %v", got)
	}
	// A .conanrc without conan_home names none; one nearer the conanfile wins.
	files = write(t, map[string]string{
		".conanrc":                  "conan_home=./outer\n",
		"app/.conanrc":              "core:non_interactive=True\n",
		"app/conanfile.py":          "",
		"outer/remotes.json":        `{"remotes": [{"name": "outer", "url": "https://outer.test", "verify_ssl": true}]}`,
		"lib/sub/conanfile.txt":     "",
		"lib/.conanrc":              "conan_home=../lib-home\n",
		"lib-home/remotes.json":     `{"remotes": [{"name": "lib", "url": "https://lib.test", "verify_ssl": true}]}`,
		"other/.conan/remotes.json": `{"remotes": [{"name": "none", "url": "https://none.test", "verify_ssl": true}]}`,
	})
	config = NewDiscoverer(environment(nil), t.TempDir()).Discover(files)
	if got := conanOrder(config, conanTarget("zlib", "1.3.1", "")); !reflect.DeepEqual(got, []string{"https://lib.test?"}) {
		t.Errorf("nearest .conanrc: %v", got)
	}
}

// conanRemote serves Conan's REST API v2 for libcurl (8.4.0 and a user/channel
// build), zlib and openssl, answering only to the token its authenticate
// endpoint hands out for mona's password.
type conanRemote struct {
	*httptest.Server
	mu     sync.Mutex
	asked  []string
	scheme []string
}

func newConanRemote(t *testing.T) *conanRemote {
	t.Helper()
	r := &conanRemote{}
	recipes := map[string]string{
		"/v2/conans/libcurl/8.4.0/_/_/revisions/rev84/files/conanfile.py": `from conan import ConanFile

class LibcurlConan(ConanFile):
    name = "libcurl"
    requires = "zlib/[>=1.2.11 <2]"
    tool_requires = "cmake/3.27.1"

    def requirements(self):
        self.requires("openssl/[>=1.1 <4]")
        self.requires("zlib/[>=1.2.11 <2]")  # the attribute's too: once
        if self.options.with_nghttp2:
            self.requires("libnghttp2/1.58.0@acme/stable")
        self.requires(f"libcurl-extra/{self.version}")
        # self.requires("c-ares/1.25.0")

    def build_requirements(self):
        self.test_requires("gtest/1.14.0")
`,
		"/v2/conans/libcurl/1.0/acme/stable/revisions/revacme/files/conanfile.py": "from conan import ConanFile\n",
		"/v2/conans/zlib/1.3.1/_/_/revisions/zrev/files/conanfile.py":             "from conan import ConanFile\n",
	}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		authorization := request.Header.Get("Authorization")
		scheme, _, _ := strings.Cut(authorization, " ")
		r.mu.Lock()
		r.asked = append(r.asked, request.URL.RequestURI())
		r.scheme = append(r.scheme, scheme)
		r.mu.Unlock()
		if request.URL.Path == "/v2/users/authenticate" {
			if authorization != "Basic "+base64.StdEncoding.EncodeToString([]byte("mona:secret")) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("conan-token\n"))
			return
		}
		if authorization != "Bearer conan-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch path := request.URL.Path; {
		case path == "/v2/conans/search" && request.URL.Query().Get("q") == "libcurl/*":
			w.Write([]byte(`{"results": ["libcurl/7.88.1", "libcurl/8.4.0", "libcurl/8.5.0-rc1", "libcurl/9.0.0", "libcurl/8.9.0@acme/stable", "libcurl/8.8.0@acme", "libcurl-extra/8.6.0"]}`))
		case path == "/v2/conans/search":
			w.Write([]byte(`{"results": []}`))
		case path == "/v2/conans/libcurl/8.4.0/_/_/latest":
			w.Write([]byte(`{"revision": "rev84", "time": "2024-01-01T00:00:00Z"}`))
		case path == "/v2/conans/libcurl/1.0/acme/stable/latest":
			w.Write([]byte(`{"revision": "revacme", "time": "2024-01-01T00:00:00Z"}`))
		case recipes[path] != "":
			w.Write([]byte(recipes[path]))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(r.Close)
	return r
}

func (r *conanRemote) take() (asked, scheme []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	asked, scheme, r.asked, r.scheme = r.asked, r.scheme, nil, nil
	return asked, scheme
}

// conanHome is a machine whose Conan home lists remote, with mona's login in
// credentials.json.
func conanHome(t *testing.T, remote string) string {
	t.Helper()
	home := t.TempDir()
	put(t, filepath.Join(home, ".conan2", "remotes.json"), `{"remotes": [{"name": "acme", "url": "`+remote+`", "verify_ssl": true}]}`)
	put(t, filepath.Join(home, ".conan2", "credentials.json"), `{"credentials": [{"remote": "acme", "user": "mona", "password": "secret"}]}`)
	return home
}

// A range is resolved against the remote's search (the newest release it admits
// for the same user and channel), the recipe's latest revision named, and its
// conanfile.py read: its requires - attribute and self.requires() calls, with
// their user and channel - answer, not its tool or test requirements or a
// reference built at run time; the repository's conan.lock pins what it pins,
// with the revision, which is then read without asking for the latest. The
// remote answers 401 until the login in credentials.json is exchanged for a
// token at /v2/users/authenticate (Basic), which is sent as a Bearer token from
// then on, and asked for once.
//
// Verifies: REQ-SUP-076, REQ-AUTH-035, REQ-CPP-011
func TestConanRecipeDependencies(t *testing.T) {
	remote := newConanRemote(t)
	home := conanHome(t, remote.URL)
	files := write(t, map[string]string{
		"conanfile.txt": "[requires]\nlibcurl/[>=8 <9]\n",
		"conan.lock":    `{"version": "0.5", "requires": ["openssl/3.2.1#orev%1700000000.0", "zlib/1.3.1#zrev%1700000000.0", "libnghttp2/1.58.0#nrev%1.0"]}`,
	})
	config := NewDiscoverer(environment(nil), home).Discover(files)
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, nil), nil)
	got := c.Dependencies(conanTarget("libcurl", "[>=8 <9]", ""))
	want := []lang.Target{
		{Ecosystem: Conan, Package: "libnghttp2", Version: "1.58.0", Pinned: true, Registry: "@acme/stable"},
		{Ecosystem: Conan, Package: "openssl", Version: "3.2.1", Requested: "[>=1.1 <4]", Pinned: true, Registry: "#orev"},
		{Ecosystem: Conan, Package: "zlib", Version: "1.3.1", Requested: "[>=1.2.11 <2]", Pinned: true, Registry: "#zrev"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	asked, scheme := remote.take()
	wantAsked := []string{
		"/v2/conans/search?q=libcurl%2F%2A", "/v2/users/authenticate", "/v2/conans/search?q=libcurl%2F%2A",
		"/v2/conans/libcurl/8.4.0/_/_/latest", "/v2/conans/libcurl/8.4.0/_/_/revisions/rev84/files/conanfile.py",
	}
	if !reflect.DeepEqual(asked, wantAsked) || !reflect.DeepEqual(scheme, []string{"", "Basic", "Bearer", "Bearer", "Bearer"}) {
		t.Errorf("asked %v %v", asked, scheme)
	}
	// The lock's revision is read as it is.
	if got := c.Dependencies(conanTarget("zlib", "[>=1.2 <2]", "")); len(got) != 0 {
		t.Errorf("zlib: %v", got)
	}
	if asked, _ := remote.take(); !reflect.DeepEqual(asked, []string{"/v2/conans/zlib/1.3.1/_/_/revisions/zrev/files/conanfile.py"}) {
		t.Errorf("zlib asked %v", asked)
	}
	c.Dependencies(conanTarget("libcurl", "1.0", "@acme/stable"))
	if asked, _ := remote.take(); !reflect.DeepEqual(asked, []string{"/v2/conans/libcurl/1.0/acme/stable/latest",
		"/v2/conans/libcurl/1.0/acme/stable/revisions/revacme/files/conanfile.py"}) {
		t.Errorf("user and channel: asked %v", asked)
	}
	if _, lookup := ask(t, c, conanTarget("libcurl", "[>=10]", "")); lookup.Answer != trace.NoAnswer || !strings.Contains(lookup.Reason, "lists no version") {
		t.Errorf("no version admitted: %+v", lookup)
	}
	if _, lookup := ask(t, c, conanTarget("missing", "1.0", "")); !strings.Contains(lookup.Reason, "404") {
		t.Errorf("missing: %+v", lookup)
	}
}

// Without a login for the remote, its 401 is the answer; with Conan's
// auth_remote.py plugin in the home, a note says the plugin is not run. A login
// is never sent to a remote reached over plain http elsewhere than this machine.
//
// Verifies: REQ-AUTH-035, REQ-TRC-017
func TestConanLoginIsSentOnlyToTheAuthenticateEndpoint(t *testing.T) {
	remote := newConanRemote(t)
	home := t.TempDir()
	put(t, filepath.Join(home, ".conan2", "remotes.json"), `{"remotes": [{"name": "acme", "url": "`+remote.URL+`", "verify_ssl": true}]}`)
	put(t, filepath.Join(home, ".conan2", "extensions", "plugins", "auth_remote.py"), "def auth_remote_plugin(remote, user=None):\n    return None, None\n")
	c := NewClient(NewDiscoverer(environment(nil), home).Discover(nil), t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, nil), nil)
	notes := notesOf(t, c, conanTarget("zlib", "1.3.1", ""))
	if len(notes) != 1 || !strings.HasPrefix(notes[0], trace.NoteHelperNotRun+": Conan remote "+remote.URL+" wants a login") {
		t.Errorf("notes %v", notes)
	}
	if asked, _ := remote.take(); !reflect.DeepEqual(asked, []string{"/v2/conans/zlib/1.3.1/_/_/latest"}) {
		t.Errorf("asked %v", asked)
	}
	plain := "http://conan.acme.test"
	home = conanHome(t, plain)
	net := &unauthorized{}
	c = NewClient(NewDiscoverer(environment(nil), home).Discover(nil), t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, nil), nil)
	c.http = &http.Client{Transport: net}
	if _, lookup := ask(t, c, conanTarget("zlib", "1.3.1", "")); !strings.Contains(lookup.Reason, "401") {
		t.Errorf("plain http: %+v", lookup)
	}
	if !reflect.DeepEqual(net.asked, []string{plain + "/v2/conans/zlib/1.3.1/_/_/latest"}) {
		t.Errorf("plain http asked %v", net.asked)
	}
}

// unauthorized is a transport that answers 401 to everything and records what was
// asked.
type unauthorized struct{ asked []string }

func (u *unauthorized) RoundTrip(r *http.Request) (*http.Response, error) {
	u.asked = append(u.asked, r.URL.String())
	return &http.Response{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized", Body: http.NoBody, Request: r}, nil
}

// A remote only the repository's Conan home lists is not asked, and a private
// package is not named to ConanCenter.
//
// Verifies: REQ-SUP-076, REQ-SUP-038
func TestConanRepositoryRemotesAndPrivatePackages(t *testing.T) {
	remote := newConanRemote(t)
	files := write(t, map[string]string{
		".conanrc":             "conan_home=./.conan2\n",
		"conanfile.txt":        "[requires]\nzlib/1.3.1\n",
		".conan2/remotes.json": `{"remotes": [{"name": "acme", "url": "` + remote.URL + `", "verify_ssl": true}]}`,
	})
	home := t.TempDir()
	c := NewClient(NewDiscoverer(environment(nil), home).Discover(files), t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, nil), nil)
	if _, lookup := ask(t, c, conanTarget("zlib", "1.3.1", "")); lookup.Reason != trace.ReasonUntrusted || lookup.Index != remote.URL {
		t.Errorf("repository's remote: %+v", lookup)
	}
	c = NewClient(NewDiscoverer(environment(nil), home).Discover(nil), t.TempDir(), time.Hour, 5*time.Second, nil, scope.New([]string{"conan:acme-*"}))
	if _, lookup := ask(t, c, conanTarget("acme-log", "1.0", "")); lookup.Reason != trace.ReasonPrivate {
		t.Errorf("private: %+v", lookup)
	}
	if asked, _ := remote.take(); len(asked) != 0 {
		t.Errorf("asked %v", asked)
	}
}

// Conan 2's version ranges, as its VersionRange reads them.
//
// Verifies: REQ-SUP-076
func TestConanVersionRanges(t *testing.T) {
	for _, testCase := range []struct {
		expression string
		admits     []string
		refuses    []string
	}{
		{"[>=1.0 <2]", []string{"1.0", "1.9.9"}, []string{"0.9", "2.0", "1.5-pre"}},
		{"[~1.2]", []string{"1.2.0", "1.2.9"}, []string{"1.3.0", "1.1"}},
		{"[~1]", []string{"1.0", "1.9"}, []string{"2.0"}},
		{"[^1.2]", []string{"1.2", "1.9"}, []string{"2.0", "1.1"}},
		{"[^0.2.3]", []string{"0.2.3", "0.2.9"}, []string{"0.3.0"}},
		{"[1.2.*]", []string{"1.2.0", "1.2.11"}, []string{"1.3.0"}},
		{"[*]", []string{"0.1", "10.0"}, []string{"1.0-rc1"}},
		{"[1.2.3]", []string{"1.2.3", "1.2.3.0"}, []string{"1.2.4"}},
		{"[<1 || >=2 <3]", []string{"0.5", "2.5"}, []string{"1.5", "3.0"}},
		{"[>=1.0 <2, include_prerelease]", []string{"1.5-pre"}, []string{"2.0-pre"}},
		{"[>1.10]", []string{"1.11"}, []string{"1.9", "1.10"}},
		{"[>=cci.20200101]", []string{"cci.20230101"}, []string{"cci.20190101"}},
	} {
		for _, v := range testCase.admits {
			if !conanAdmits(testCase.expression, v) {
				t.Errorf("%s does not admit %s", testCase.expression, v)
			}
		}
		for _, v := range testCase.refuses {
			if conanAdmits(testCase.expression, v) {
				t.Errorf("%s admits %s", testCase.expression, v)
			}
		}
	}
}
