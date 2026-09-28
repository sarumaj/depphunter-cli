package index

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// served is one answer of a simpleIndex: its media type and body. "{{self}}" in
// the body is the stub's own URL; media "redirect" redirects to the body.
type served struct{ media, body string }

// simpleIndex is a stub index that serves only what it is given, 404 for the rest,
// and remembers each request as "path Authorization" and each Accept header.
type simpleIndex struct {
	*httptest.Server
	mu      sync.Mutex
	asked   []string
	accepts map[string]string
}

// newSimpleIndex starts a stub; with a credential ("user:pass") it refuses every
// request that does not carry it.
func newSimpleIndex(t *testing.T, credential string, answers map[string]served) *simpleIndex {
	t.Helper()
	s := &simpleIndex{accepts: map[string]string{}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.asked = append(s.asked, strings.TrimSpace(r.URL.Path+" "+r.Header.Get("Authorization")))
		s.accepts[r.URL.Path] = r.Header.Get("Accept")
		s.mu.Unlock()
		if credential != "" && r.Header.Get("Authorization") != basicHeaderOf(credential) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		a, ok := answers[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if a.media == "redirect" {
			http.Redirect(w, r, a.body, http.StatusMovedPermanently)
			return
		}
		w.Header().Set("Content-Type", a.media)
		fmt.Fprint(w, strings.ReplaceAll(a.body, "{{self}}", "http://"+r.Host))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *simpleIndex) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.asked...)
}

const (
	simpleJSONType = "application/vnd.pypi.simple.v1+json"
	metadataBody   = "Metadata-Version: 2.1\nName: lib\nVersion: 2.0\nRequires-Dist: certifi>=2017.4.17\n" +
		"Requires-Dist: idna (<4,>=2.5)\nRequires-Dist: PySocks!=1.5.7,>=1.5.6;\n  extra == \"socks\"\n" +
		"Requires-Dist: colorama; sys_platform == \"win32\"\n\nRequires-Dist: not-a-header\n"
)

func sha(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// simpleClient asks index (a simple index URL this machine trusts) and nothing else.
func simpleClient(t *testing.T, index, cache string, store *auth.Store) *Client {
	t.Helper()
	cfg := New()
	cfg.Add(PyPI, Source{URL: index, Trusted: true})
	return NewClient(cfg, cache, time.Hour, 5*time.Second, store, nil)
}

// An index without the JSON API is read through the Simple API: the PEP 691 page
// (asked for with PEP 691's Accept header), the newest final, non-yanked release,
// and the Requires-Dist of its wheel's PEP 658 metadata - checked against its hash,
// found relative to the page where it was served after a redirect - without extras. The answer is cached like any other.
//
// Verifies: REQ-SUP-067, REQ-SUP-032
func TestSimpleAPIFallbackJSON(t *testing.T) {
	page := `{"meta":{"api-version":"1.1"},"name":"lib","files":[
		{"filename":"lib-1.0.tar.gz","url":"../../files/lib-1.0.tar.gz","hashes":{}},
		{"filename":"lib-2.0.tar.gz","url":"../../files/lib-2.0.tar.gz","hashes":{}},
		{"filename":"lib-2.0-py3-none-any.whl","url":"../../files/lib-2.0-py3-none-any.whl#sha256=00","hashes":{},
		 "core-metadata":{"sha256":"` + sha(metadataBody) + `"}},
		{"filename":"lib-2.5-py3-none-any.whl","url":"../../files/lib-2.5-py3-none-any.whl","yanked":"broken","core-metadata":true},
		{"filename":"lib-3.0rc1-py3-none-any.whl","url":"../../files/lib-3.0rc1-py3-none-any.whl","core-metadata":true},
		{"filename":"other-9.0-py3-none-any.whl","url":"../../files/other-9.0-py3-none-any.whl","core-metadata":true}]}`
	idx := newSimpleIndex(t, "", map[string]served{
		"/simple/lib/":        {"redirect", "/mirror/simple/lib/"},
		"/mirror/simple/lib/": {simpleJSONType, page},
		"/mirror/files/lib-2.0-py3-none-any.whl.metadata": {"application/octet-stream", metadataBody},
	})
	cache := t.TempDir()
	c := simpleClient(t, idx.URL+"/simple", cache, nil)
	got := names(c.Dependencies(lang.Target{Ecosystem: PyPI, Package: "Lib"}))
	if want := []string{"certifi", "colorama", "idna"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v; asked %v", got, want, idx.requests())
	}
	if want := []string{"/pypi/Lib/json", "/simple/lib/", "/mirror/simple/lib/", "/mirror/files/lib-2.0-py3-none-any.whl.metadata"}; !slices.Equal(idx.requests(), want) {
		t.Errorf("asked %v, want %v", idx.requests(), want)
	}
	if got := idx.accepts["/simple/lib/"]; got != simpleAccept {
		t.Errorf("Accept %q", got)
	}
	again := simpleClient(t, idx.URL+"/simple", cache, nil)
	if got := names(again.Dependencies(lang.Target{Ecosystem: PyPI, Package: "Lib"})); len(got) != 3 || len(idx.requests()) != 4 {
		t.Errorf("not answered from the cache: %v, asked %v", got, idx.requests())
	}
}

// A PEP 503 HTML page is read too - from an index whose JSON API path answers an
// HTML page rather than 404 - with data-dist-info-metadata and data-core-metadata,
// data-yanked, entities and absolute and host-relative links. A pinned release is
// taken even when it was yanked; a wheel's metadata is preferred to an sdist's.
//
// Verifies: REQ-SUP-067
func TestSimpleAPIFallbackHTML(t *testing.T) {
	meta := "Name: lib\nRequires-Dist: six\n"
	page := `<!DOCTYPE html><html><body><h1>Links for lib</h1>
<a href="/packages/lib-1.0.tar.gz#sha256=aa" data-core-metadata="true">lib-1.0.tar.gz</a><br/>
<a href="{{self}}/packages/lib-1.0-py2.py3-none-any.whl#sha256=bb" data-dist-info-metadata="sha256=` + sha(meta) + `" data-yanked="">lib-1.0-py2.py3-none-any.whl</a>
<a data-requires-python="&gt;=3.8" href="/packages/lib-1.1-py3-none-any.whl" data-dist-info-metadata="true">lib-1.1-py3-none-any.whl</a>
</body></html>`
	idx := newSimpleIndex(t, "", map[string]served{
		"/pypi/lib/1.0/json": {"text/html", "<html>Nexus says hello</html>"},
		"/simple/lib/":       {"text/html; charset=utf-8", page},
		"/packages/lib-1.0-py2.py3-none-any.whl.metadata": {"text/plain", meta},
		"/packages/lib-1.1-py3-none-any.whl.metadata":     {"text/plain", "Name: lib\nRequires-Dist: attrs\n"},
	})
	c := simpleClient(t, idx.URL+"/simple/", t.TempDir(), nil)
	if got := names(c.Dependencies(lang.Target{Ecosystem: PyPI, Package: "lib", Version: "1.0"})); !slices.Equal(got, []string{"six"}) {
		t.Errorf("pinned 1.0: %v, asked %v", got, idx.requests())
	}
	if got := names(c.Dependencies(lang.Target{Ecosystem: PyPI, Package: "lib"})); !slices.Equal(got, []string{"attrs"}) {
		t.Errorf("newest: %v, asked %v", got, idx.requests())
	}
}

// A release whose files advertise no metadata gets no answer, and says why; no
// archive is downloaded, and the question does not move on to another index, which
// could hold a different package of that name.
//
// Verifies: REQ-SUP-067
func TestSimpleAPIWithoutMetadata(t *testing.T) {
	idx := newSimpleIndex(t, "", map[string]served{
		"/simple/lib/": {"text/html", `<a href="../../f/lib-1.0-py3-none-any.whl">lib-1.0-py3-none-any.whl</a>`},
		// Advertised, but missing: that is no metadata either.
		"/simple/gone/": {"text/html", `<a href="/f/gone-1.0.tar.gz" data-core-metadata="true">gone-1.0.tar.gz</a>`},
	})
	pypi := newFeed(t, map[string]string{"/pypi/lib/json": pypiJSON("wrong")})
	asPublic(t, PyPI, pypi)
	cfg := New()
	cfg.Add(PyPI, Source{URL: idx.URL + "/simple", Trusted: true, Kind: Additive})
	c := newClient(t, cfg)
	for _, pkg := range []string{"lib", "gone"} {
		got, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: pkg})
		if len(got) != 0 || !strings.Contains(l.Reason, "no metadata file") {
			t.Errorf("%s: %v, reason %q", pkg, got, l.Reason)
		}
	}
	for _, r := range idx.requests() {
		if strings.HasSuffix(r, ".whl") || strings.HasSuffix(r, ".tar.gz") || r == "/f/lib-1.0-py3-none-any.whl.metadata" {
			t.Errorf("downloaded %s", r)
		}
	}
	if len(pypi.paths()) != 0 {
		t.Errorf("PyPI was asked: %v", pypi.paths())
	}
}

// A package the simple index does not have (404 page), or has without the pinned
// release, moves on to PyPI as a JSON API 404 does.
//
// Verifies: REQ-SUP-067, REQ-SUP-063
func TestSimpleAPINotFoundMovesOn(t *testing.T) {
	idx := newSimpleIndex(t, "", map[string]served{
		"/simple/lib/": {simpleJSONType, `{"files":[{"filename":"lib-1.0.tar.gz","url":"lib-1.0.tar.gz","core-metadata":true}]}`},
	})
	pypi := newFeed(t, map[string]string{"/pypi/requests/json": pypiJSON("certifi"), "/pypi/lib/2.0/json": pypiJSON("six")})
	asPublic(t, PyPI, pypi)
	cfg := New()
	cfg.Add(PyPI, Source{URL: idx.URL + "/simple", Trusted: true, Kind: Additive})
	c := newClient(t, cfg)
	for _, tc := range []struct{ pkg, version, want string }{{"requests", "", "certifi"}, {"lib", "2.0", "six"}} {
		got, l := ask(t, c, lang.Target{Ecosystem: PyPI, Package: tc.pkg, Version: tc.version})
		if !slices.Equal(got, []string{tc.want}) || l.Index != pypi.URL {
			t.Errorf("%s: %v from %s, asked %v", tc.pkg, got, l.Index, idx.requests())
		}
	}
}

// The credential this machine keeps for a path-scoped index goes with the page and
// with the metadata files below the same path; a file on another host gets that
// host's credential - here none.
//
// Verifies: REQ-SUP-067, REQ-AUTH-026, REQ-SUP-033
func TestSimpleAPICredentials(t *testing.T) {
	other := newSimpleIndex(t, "", map[string]served{
		"/cdn/ext-1.0-py3-none-any.whl.metadata": {"text/plain", "Requires-Dist: wrapt\n"},
	})
	idx := newSimpleIndex(t, "ci:secret", map[string]served{
		"/team/simple/lib/":                             {simpleJSONType, `{"files":[{"filename":"lib-1.0-py3-none-any.whl","url":"../../files/lib-1.0-py3-none-any.whl","core-metadata":true}]}`},
		"/team/simple/ext/":                             {simpleJSONType, `{"files":[{"filename":"ext-1.0-py3-none-any.whl","url":"` + other.URL + `/cdn/ext-1.0-py3-none-any.whl","core-metadata":true}]}`},
		"/team/files/lib-1.0-py3-none-any.whl.metadata": {"text/plain", "Requires-Dist: certifi\n"},
	})
	asPublic(t, PyPI, newFeed(t, nil))
	e := env(map[string]string{
		"UV_DEFAULT_INDEX":       "corp=" + idx.URL + "/team/simple",
		"UV_INDEX_CORP_USERNAME": "ci", "UV_INDEX_CORP_PASSWORD": "secret",
	})
	home := t.TempDir()
	store := auth.Read(home, e)
	d := NewDiscoverer(e, home)
	d.Config().Credentials(store)
	c := NewClient(d.Discover(nil), t.TempDir(), time.Hour, 5*time.Second, store, nil)
	if got := names(c.Dependencies(lang.Target{Ecosystem: PyPI, Package: "lib"})); !slices.Equal(got, []string{"certifi"}) {
		t.Errorf("lib: %v, asked %v", got, idx.requests())
	}
	cred := " " + basicHeaderOf("ci:secret")
	if want := []string{"/team/pypi/lib/json" + cred, "/team/simple/lib/" + cred, "/team/files/lib-1.0-py3-none-any.whl.metadata" + cred}; !slices.Equal(idx.requests(), want) {
		t.Errorf("asked %v, want %v", idx.requests(), want)
	}
	if got := names(c.Dependencies(lang.Target{Ecosystem: PyPI, Package: "ext"})); !slices.Equal(got, []string{"wrapt"}) {
		t.Errorf("ext: %v", got)
	}
	if got := other.requests(); !slices.Equal(got, []string{"/cdn/ext-1.0-py3-none-any.whl.metadata"}) {
		t.Errorf("the other host was sent %v", got)
	}
}

// PyPI itself has the JSON API: a 404 there is the answer, and its simple pages are
// not asked.
//
// Verifies: REQ-SUP-023, REQ-SUP-067
func TestPublicPyPIIsNotAskedForSimplePages(t *testing.T) {
	pypi := newFeed(t, map[string]string{"/pypi/requests/json": pypiJSON("certifi")})
	asPublic(t, PyPI, pypi)
	c := newClient(t, New())
	if got, _ := ask(t, c, lang.Target{Ecosystem: PyPI, Package: "requests"}); !slices.Equal(got, []string{"certifi"}) {
		t.Errorf("requests: %v", got)
	}
	if got, _ := ask(t, c, lang.Target{Ecosystem: PyPI, Package: "nothing"}); len(got) != 0 {
		t.Errorf("nothing: %v", got)
	}
	if want := []string{"/pypi/requests/json", "/pypi/nothing/json"}; !slices.Equal(pypi.paths(), want) {
		t.Errorf("asked %v, want %v", pypi.paths(), want)
	}
}

// Verifies: REQ-SUP-067
func TestPEP440Order(t *testing.T) {
	ordered := []string{"0.9", "1.0.dev1", "1.0a1.dev1", "1.0a1", "1.0b2", "1.0rc1", "1.0", "1.0.post1.dev1", "1.0.post1", "1.1", "1.10", "1!0.1"}
	for i := range ordered {
		for j := range ordered {
			a, okA := parsePEP440(ordered[i])
			b, okB := parsePEP440(ordered[j])
			if !okA || !okB {
				t.Fatalf("%s or %s not parsed", ordered[i], ordered[j])
			}
			if got, want := a.compare(b), cmp.Compare(i, j); got != want {
				t.Errorf("compare(%s, %s) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
	for _, same := range [][2]string{{"1.0", "1.0.0"}, {"v1.0-RC.1", "1.0rc1"}, {"1.0-1", "1.0.post1"}, {"1.0alpha", "1.0a0"}, {"1.0+local.7", "1.0"}} {
		a, _ := parsePEP440(same[0])
		b, _ := parsePEP440(same[1])
		if a.compare(b) != 0 {
			t.Errorf("%s != %s", same[0], same[1])
		}
	}
	for v, pre := range map[string]bool{"1.0": false, "1.0.post2": false, "1.0rc1": true, "1.0.dev0": true, "2.0b1.post1": true} {
		if p, _ := parsePEP440(v); p.prerelease() != pre {
			t.Errorf("%s prerelease %v", v, p.prerelease())
		}
	}
	if _, ok := parsePEP440("not.a.version"); ok {
		t.Error("parsed not.a.version")
	}
}

// Verifies: REQ-SUP-067
func TestDistVersion(t *testing.T) {
	for file, want := range map[string]string{
		"zope.interface-6.0-cp311-cp311-manylinux_2_17_x86_64.whl": "6.0",
		"zope_interface-6.0-1-cp311-cp311-win32.whl":               "6.0",
		"zope.interface-6.0.tar.gz":                                "6.0",
		"Zope-Interface-6.0b1.zip":                                 "6.0b1",
		"zope-interface-extra-1.0.tar.gz":                          "",
		"zope.interface-6.0-py3.11.egg":                            "",
		"zope.interface-6.0.whl":                                   "",
	} {
		got, ok := distVersion(file, pypiName("zope.interface"))
		if got != want || ok != (want != "") {
			t.Errorf("%s: %q %v, want %q", file, got, ok, want)
		}
	}
	for file, want := range map[string]string{
		"https://x/f/a-1.0.whl#sha256=ab":   "https://x/f/a-1.0.whl.metadata",
		"https://x/f/a-1.0.whl?sig=1#sha=2": "https://x/f/a-1.0.whl.metadata?sig=1",
	} {
		if got := metadataURL(file); got != want {
			t.Errorf("%s: %s", file, got)
		}
	}
}

// Metadata that does not match the hash the page gives for it is not read.
//
// Verifies: REQ-SUP-067
func TestSimpleAPIMetadataHashMismatch(t *testing.T) {
	idx := newSimpleIndex(t, "", map[string]served{
		"/simple/lib/":               {simpleJSONType, `{"files":[{"filename":"lib-1.0.tar.gz","url":"/f/lib-1.0.tar.gz","core-metadata":{"sha256":"` + sha("other") + `"}}]}`},
		"/f/lib-1.0.tar.gz.metadata": {"text/plain", "Requires-Dist: evil\n"},
	})
	got, l := ask(t, simpleClient(t, idx.URL+"/simple", t.TempDir(), nil), lang.Target{Ecosystem: PyPI, Package: "lib"})
	if len(got) != 0 || !strings.Contains(l.Reason, "does not match") {
		t.Errorf("%v, reason %q", got, l.Reason)
	}
}
