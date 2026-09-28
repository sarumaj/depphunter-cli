package index

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scope"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// stubIndex serves what each ecosystem's index serves, so the client can be checked
// without the network.
func stubIndex(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var asked []string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if r.Header.Get("Authorization") == "" && r.URL.Path == "/private/1.0.0" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/example.com/mod/@v/v1.2.3.mod":
			fmt.Fprint(w, "module example.com/mod\n\ngo 1.22\n\nrequire (\n\tgithub.com/direct/dep v1.0.0\n\tgithub.com/indirect/dep v1.5.0 // indirect\n)\n")
		case "/react/18.3.1":
			fmt.Fprint(w, `{"name":"react","dependencies":{"loose-envify":"^1.1.0"}}`)
		case "/react/latest":
			fmt.Fprint(w, `{"name":"react","dependencies":{"loose-envify":"^1.1.0","js-tokens":"^4"}}`)
		case "/pypi/requests/2.31.0/json":
			fmt.Fprint(w, `{"info":{"requires_dist":["certifi (>=2017.4.17)","urllib3 (<3,>=1.21.1)","PySocks ; extra == 'socks'"]}}`)
		case "/private/1.0.0":
			fmt.Fprint(w, `{"name":"private","dependencies":{"secret-dep":"1.0.0"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, &asked
}

func clientFor(t *testing.T, ecosystem, url, home string) *Client {
	t.Helper()
	config := New()
	config.Add(ecosystem, Source{URL: url, Trusted: true})
	return NewClient(config, t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, nil), nil)
}

func names(dependencies []lang.Target) []string {
	var out []string
	for _, d := range dependencies {
		out = append(out, d.Package)
	}
	sort.Strings(out)
	return out
}

// Verifies: REQ-SUP-021, REQ-SUP-029
func TestGoModuleDependencies(t *testing.T) {
	server, _ := stubIndex(t)
	c := clientFor(t, Go, server.URL, "")
	got := names(c.Dependencies(lang.Target{Ecosystem: Go, Package: "example.com/mod", Version: "v1.2.3"}))
	// An indirect requirement belongs to something else's go.mod, not to this module.
	if len(got) != 1 || got[0] != "github.com/direct/dep" {
		t.Errorf("got %v, want the direct requirement only", got)
	}
	// Without a version the proxy has no document to serve, and nothing is asked.
	if dependencies := c.Dependencies(lang.Target{Ecosystem: Go, Package: "example.com/mod"}); len(dependencies) != 0 {
		t.Errorf("got %v for a module with no version", names(dependencies))
	}
}

// Verifies: REQ-SUP-022
func TestNpmDependencies(t *testing.T) {
	server, asked := stubIndex(t)
	c := clientFor(t, NPM, server.URL, "")
	if got := names(c.Dependencies(lang.Target{Ecosystem: NPM, Package: "react", Version: "18.3.1"})); len(got) != 1 || got[0] != "loose-envify" {
		t.Errorf("got %v", got)
	}
	// A range names no document, so the registry is asked for the current version.
	if got := names(c.Dependencies(lang.Target{Ecosystem: NPM, Package: "react", Version: "^18.0.0"})); len(got) != 2 {
		t.Errorf("got %v, want what latest requires", got)
	}
	if len(*asked) != 2 || (*asked)[1] != "/react/latest" {
		t.Errorf("asked %v", *asked)
	}
}

// Verifies: REQ-SUP-023
func TestPyPIDependencies(t *testing.T) {
	server, _ := stubIndex(t)
	config := New()
	config.Add(PyPI, Source{URL: server.URL + "/simple", Trusted: true})
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil, nil)
	got := names(c.Dependencies(lang.Target{Ecosystem: PyPI, Package: "requests", Version: "2.31.0"}))
	// An extra's dependency is installed only when that extra is asked for.
	if len(got) != 2 || got[0] != "certifi" || got[1] != "urllib3" {
		t.Errorf("got %v, want certifi and urllib3", got)
	}
}

// Verifies: REQ-SUP-019
func TestAnIndexOnlyTheRepositoryNamesIsNotAsked(t *testing.T) {
	server, asked := stubIndex(t)
	config := New()
	config.Add(NPM, Source{URL: server.URL}) // as a repository's .npmrc would
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil, nil)
	if dependencies := c.Dependencies(lang.Target{Ecosystem: NPM, Package: "react", Version: "18.3.1"}); len(dependencies) != 0 {
		t.Errorf("got %v from an index nothing here vouches for", names(dependencies))
	}
	if len(*asked) != 0 {
		t.Errorf("the index was asked anyway: %v", *asked)
	}
}

// Verifies: REQ-SUP-033
func TestCredentialsGoToTheHostTheyWereWrittenFor(t *testing.T) {
	server, _ := stubIndex(t)
	home := t.TempDir()
	host := server.Listener.Addr().String()
	npmrc := fmt.Sprintf("//%s/:_authToken=secret-token\n", host)
	if err := os.WriteFile(filepath.Join(home, ".npmrc"), []byte(npmrc), 0o600); err != nil {
		t.Fatal(err)
	}
	c := clientFor(t, NPM, server.URL, home)
	if got := names(c.Dependencies(lang.Target{Ecosystem: NPM, Package: "private", Version: "1.0.0"})); len(got) != 1 {
		t.Errorf("got %v, want the private package's dependency", got)
	}
	// The same request without the token is refused by the stub, which is what
	// proves the header was sent.
	plain := clientFor(t, NPM, server.URL, t.TempDir())
	if got := plain.Dependencies(lang.Target{Ecosystem: NPM, Package: "private", Version: "1.0.0"}); len(got) != 0 {
		t.Errorf("got %v without credentials", names(got))
	}
}

// Verifies: REQ-SUP-032
func TestAnswersAreCached(t *testing.T) {
	server, asked := stubIndex(t)
	directory := t.TempDir()
	config := New()
	config.Add(Go, Source{URL: server.URL, Trusted: true})
	target := lang.Target{Ecosystem: Go, Package: "example.com/mod", Version: "v1.2.3"}

	first := NewClient(config, directory, time.Hour, 5*time.Second, nil, nil)
	first.Dependencies(target)
	// A second client, a second run: the answer is on disk.
	second := NewClient(config, directory, time.Hour, 5*time.Second, nil, nil)
	if got := names(second.Dependencies(target)); len(got) != 1 {
		t.Errorf("got %v from the cache", got)
	}
	if len(*asked) != 1 {
		t.Errorf("the index was asked %d times", len(*asked))
	}
	// An expired answer is asked for again. The entry is aged rather than the time
	// to live shortened: Windows reads a clock that ticks every fifteen
	// milliseconds, so an answer written and read inside one test is the same
	// instant there, and no time to live short of zero expires it.
	age(t, directory, 2*time.Hour)
	third := NewClient(config, directory, time.Hour, 5*time.Second, nil, nil)
	third.Dependencies(target)
	if len(*asked) != 2 {
		t.Errorf("a stale answer was reused: %v", *asked)
	}
}

// age moves every cached answer in directory that far into the past.
func age(t *testing.T, directory string, by time.Duration) {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil || len(found) == 0 {
		t.Fatalf("no cached answers in %s: %v", directory, err)
	}
	for _, name := range found {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		// Read as a bag of fields: what is aged is the timestamp, and what is kept
		// is whatever internal/store wrote beside it.
		var e map[string]json.RawMessage
		if err := json.Unmarshal(data, &e); err != nil {
			t.Fatal(err)
		}
		var at time.Time
		if err := json.Unmarshal(e["at"], &at); err != nil {
			t.Fatal(err)
		}
		if e["at"], err = json.Marshal(at.Add(-by)); err != nil {
			t.Fatal(err)
		}
		if data, err = json.Marshal(e); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Verifies: REQ-SUP-038
func TestAPrivatePackageIsNotNamedToAPublicIndex(t *testing.T) {
	server, asked := stubIndex(t)
	// The stub stands in for the ecosystem's public index, which is what makes
	// asking it a disclosure.
	public[Go] = server.URL
	t.Cleanup(func() { public[Go] = "https://proxy.golang.org" })

	config := New()
	config.Add(Go, Source{URL: server.URL, Trusted: true})
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil,
		scope.New([]string{"go:example.com/*"}))

	if dependencies := c.Dependencies(lang.Target{Ecosystem: Go, Package: "example.com/mod", Version: "v1.2.3"}); len(dependencies) != 0 {
		t.Errorf("answered %v for a private module", names(dependencies))
	}
	// Not "asked and ignored": not asked. The request is what would say the module
	// exists, and to whom.
	for _, path := range *asked {
		t.Errorf("asked the public index for %s", path)
	}
}

// cSpell: words Companys
//
// Verifies: REQ-SUP-039
func TestAPrivatePackageIsStillAskedOfTheCompanysOwnIndex(t *testing.T) {
	server, asked := stubIndex(t)
	// The machine's own configuration points Go at an internal proxy, which is not
	// the ecosystem's public one: asking it discloses nothing that is not already
	// inside the organization.
	c := clientFor(t, Go, server.URL, "")
	c.private = scope.New([]string{"go:example.com/*"})

	got := names(c.Dependencies(lang.Target{Ecosystem: Go, Package: "example.com/mod", Version: "v1.2.3"}))
	if len(got) != 1 || got[0] != "github.com/direct/dep" {
		t.Errorf("got %v, want the module's own requirement", got)
	}
	if len(*asked) == 0 {
		t.Error("the internal proxy was never asked")
	}
}

// TestTheReportSaysWhoAnswered checks the account the client gives of itself. Most
// of these answer nothing, and on the map a package nothing answered for looks
// exactly like one that depends on nothing; the report is where they are told apart.
//
// Verifies: REQ-SUP-028, REQ-SUP-029, REQ-TRC-005, REQ-TRC-006, REQ-TRC-007
func TestTheReportSaysWhoAnswered(t *testing.T) {
	server, _ := stubIndex(t)
	// The stub stands in for the Go ecosystem's public index, which is what makes
	// naming a private module to it a disclosure.
	public[Go] = server.URL
	t.Cleanup(func() { public[Go] = "https://proxy.golang.org" })

	config := New()
	config.Add(Go, Source{URL: server.URL, Trusted: true, Origin: OriginMachine})
	config.Add(NPM, Source{URL: server.URL, Origin: OriginProject}) // as a repository's .npmrc would
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil, scope.New([]string{"go:private.example/*"}))
	report := trace.New(1, true, nil, nil)
	c.Trace(report)

	for _, ask := range []lang.Target{
		{Ecosystem: Go, Package: "example.com/mod", Version: "v1.2.3"},
		{Ecosystem: Go, Package: "example.com/mod", Version: "v1.2.3"}, // the same question twice
		{Ecosystem: Go, Package: "private.example/billing", Version: "v1.0.0"},
		{Ecosystem: Go, Package: "example.com/unversioned"},
		{Ecosystem: NPM, Package: "react", Version: "18.3.1"},
		{Ecosystem: Maven, Package: "com.google.guava", Version: "33.0.0-jre"},
		{Ecosystem: "actions", Package: "actions/checkout", Version: "v4"},
		{Ecosystem: Go, Package: "example.com/missing", Version: "v9.9.9"},
	} {
		c.Dependencies(ask)
	}

	got := map[string]trace.Lookup{}
	for _, l := range report.Lookups {
		if have, ok := got[l.Package]; ok && have.Answer == trace.FromIndex {
			continue // the first answer for a package asked twice
		}
		got[l.Package] = l
	}
	for _, want := range []struct {
		packageName string
		answer      trace.Answer
		reason      string
	}{
		{"example.com/mod", trace.FromIndex, ""},
		{"private.example/billing", trace.NoAnswer, trace.ReasonPrivate},
		{"example.com/unversioned", trace.NoAnswer, trace.ReasonNoVersion},
		{"react", trace.NoAnswer, trace.ReasonUntrusted},
		{"com.google.guava", trace.NoAnswer, trace.ReasonUnsupported},
		{"actions/checkout", trace.NoAnswer, trace.ReasonNoIndex},
	} {
		l, ok := got[want.packageName]
		switch {
		case !ok:
			t.Errorf("%s was not recorded at all", want.packageName)
		case l.Answer != want.answer || l.Reason != want.reason:
			t.Errorf("%s: %q / %q, want %q / %q", want.packageName, l.Answer, l.Reason, want.answer, want.reason)
		}
	}
	// A question that was put and came back empty-handed carries what was sent and
	// what came back, which is the difference between "the index said no" and "the
	// index was never asked".
	if missing := got["example.com/missing"]; len(missing.Requests) != 1 ||
		!strings.Contains(missing.Reason, "404") {
		t.Errorf("a 404 was recorded as %+v", missing)
	}
	// Asking twice is one question, and the report says so rather than implying two
	// round trips.
	memo := 0
	for _, l := range report.Lookups {
		if l.Answer == trace.FromMemo {
			memo++
		}
	}
	if memo != 1 {
		t.Errorf("the repeated question was recorded %d times as already asked", memo)
	}
	// One for example.com/mod and one for example.com/missing. Everything else was
	// declined, answered from memory, or belongs to an ecosystem with nothing to ask.
	if report.Totals.Requests != 2 {
		t.Errorf("requests: %d, want 2", report.Totals.Requests)
	}
}

// Verifies: REQ-SUP-032
func TestAFailedLookupIsAskedAgainLater(t *testing.T) {
	down := true
	var asked int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked++
		if down {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, "module example.com/mod\n\nrequire github.com/direct/dep v1.0.0\n")
	}))
	t.Cleanup(server.Close)
	c := clientFor(t, Go, server.URL, "")
	target := lang.Target{Ecosystem: Go, Package: "example.com/mod", Version: "v1.2.3"}

	if got := c.Dependencies(target); len(got) != 0 {
		t.Fatalf("an index that is down answered %v", got)
	}
	// Straight away, the same question is not put to an index that just failed.
	down = false
	c.Dependencies(target)
	if asked != 1 {
		t.Errorf("asked %d times within the retry interval", asked)
	}
	// Once it has passed - a later --watch cycle - the index is asked again, and
	// the package gets the dependencies it has.
	for k := range c.failed {
		c.failed[k] = time.Now().Add(-2 * failRetry)
	}
	if got := names(c.Dependencies(target)); len(got) != 1 {
		t.Errorf("after the index recovered: %v (asked %d times)", got, asked)
	}
}

// Verifies: REQ-PY-015
func TestAPackageInstalledFromElsewhereIsNotAsked(t *testing.T) {
	server, asked := stubIndex(t)
	config := New()
	config.Add(PyPI, Source{URL: server.URL, Trusted: true})
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil, scope.New(nil))
	report := trace.New(1, true, nil, nil)
	c.Trace(report)

	if dependencies := c.Dependencies(lang.Target{Ecosystem: PyPI, Package: "acme-core", Version: "1.4.0", Origin: "file:///src/acme-core"}); len(dependencies) != 0 {
		t.Errorf("answered %v for a package no index has", names(dependencies))
	}
	for _, path := range *asked {
		t.Errorf("asked the index for %s", path)
	}
	report.Finish()
	if got := report.Lookups; len(got) != 1 || got[0].Reason != trace.ReasonInstalled {
		t.Errorf("reported %+v, want one lookup declined as installed", got)
	}
}

// GOAUTH decides whether the go command sends the netrc's credential to a module
// proxy: unset or a list naming netrc sends it, off or a list without netrc (a
// "git" or command entry, which depphunter does not run) does not. The go env file
// says so as well as the environment does.
//
// Verifies: REQ-AUTH-029, REQ-AUTH-002
func TestGOAUTHDecidesWhetherTheNetrcReachesTheProxy(t *testing.T) {
	const module = "/example.com/mod/@v/v1.0.0.mod"
	var mu sync.Mutex
	var headers []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		headers = append(headers, r.Header.Get("Authorization"))
		mu.Unlock()
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, "module example.com/mod\n\nrequire example.com/dep v1.0.0\n")
	}))
	t.Cleanup(server.Close)
	home := t.TempDir()
	netrc := filepath.Join(home, ".netrc")
	if err := os.WriteFile(netrc, []byte("machine 127.0.0.1 login gopher password s3cr3t\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	goenv := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(goenv, []byte("GOAUTH=off\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		variables map[string]string
		sent      bool
	}{
		{map[string]string{}, true},
		{map[string]string{"GOAUTH": "netrc"}, true},
		{map[string]string{"GOAUTH": "git /src;netrc"}, true},
		{map[string]string{"GOAUTH": "off"}, false},
		{map[string]string{"GOAUTH": "git /src"}, false},
		{map[string]string{"GOAUTH": "my-credential-command --flag"}, false},
		{map[string]string{"GOENV": goenv}, false},
	} {
		test.variables["GOPROXY"] = server.URL
		mu.Lock()
		headers = nil
		mu.Unlock()
		store := auth.Read(home, environment(test.variables))
		c := NewClient(NewDiscoverer(environment(test.variables), "").Discover(nil), t.TempDir(), time.Hour, 5*time.Second, store, nil)
		got := c.Dependencies(lang.Target{Ecosystem: Go, Package: "example.com/mod", Version: "v1.0.0"})
		if test.sent != (len(got) == 1) {
			t.Errorf("%v: got %v, want the credential sent: %v", test.variables, names(got), test.sent)
		}
		mu.Lock()
		if !test.sent && (len(headers) != 1 || headers[0] != "") {
			t.Errorf("%v: headers %q", test.variables, headers)
		}
		mu.Unlock()
		// Whatever GOAUTH says, it is the go command's: another ecosystem's request to
		// the same host still carries the netrc credential.
		request := httptest.NewRequest(http.MethodGet, server.URL+module, nil)
		store.Apply(request)
		if request.Header.Get("Authorization") == "" {
			t.Errorf("%v: Apply left the netrc out", test.variables)
		}
	}
}
