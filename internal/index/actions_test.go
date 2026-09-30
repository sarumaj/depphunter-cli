package index

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
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

// cSpell: ignore octo

// githubStub serves files by path and query, and records each request with the
// Authorization and Accept it carried.
type githubStub struct {
	*httptest.Server
	mu    sync.Mutex
	asked []string
}

func newGitHubStub(t *testing.T, bodies map[string]string) *githubStub {
	t.Helper()
	s := &githubStub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		if r.URL.RawQuery != "" {
			key += "?" + r.URL.RawQuery
		}
		s.mu.Lock()
		s.asked = append(s.asked, key+" "+r.Header.Get("Authorization")+" "+r.Header.Get("Accept"))
		s.mu.Unlock()
		body, ok := bodies[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *githubStub) take() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.asked
	s.asked = nil
	return out
}

// withRaw points raw.githubusercontent.com at url for the rest of the test.
func withRaw(t *testing.T, url string) {
	t.Helper()
	was := githubRaw
	githubRaw = url
	t.Cleanup(func() { githubRaw = was })
}

func actionNames(dependencies []lang.Target) []string {
	out := []string{}
	for _, d := range dependencies {
		name := d.Ecosystem + ":" + d.Package + "@" + d.Version
		if d.Registry != "" {
			name += " " + d.Registry
		}
		if d.Pinned {
			name += " pinned"
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// The GitHub instances this machine works with - GITHUB_API_URL's, GH_HOST's and
// the hosts of gh's hosts.yml - are asked before github.com, each at its API.
//
// Verifies: REQ-SUP-077, REQ-SUP-064
func TestActionSources(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "gh", "hosts.yml"), "github.com:\n    oauth_token: x\nacme.ghe.com:\n    user: mona\n")
	config := discoverOn(home, "linux", map[string]string{
		"GITHUB_API_URL": "https://ghe.acme.test/api/v3", "GH_HOST": "other.acme.test",
	})
	want := []string{"https://ghe.acme.test/api/v3", "https://other.acme.test/api/v3", "https://api.acme.ghe.com", "https://api.github.com"}
	if got := order(config, Actions, "actions/checkout", ""); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	config = discoverOn(t.TempDir(), "windows", map[string]string{"GITHUB_API_URL": "https://api.github.com"})
	if got := order(config, Actions, "actions/checkout", ""); !reflect.DeepEqual(got, []string{"https://api.github.com"}) {
		t.Errorf("github.com alone: %v", got)
	}
}

// Without a credential for api.github.com, a github.com action's files are read
// from raw.githubusercontent.com: a composite action's steps (a sub-directory's
// action.yml, action.yaml when there is no action.yml), a Docker action's image
// and its Dockerfile's FROM, a reusable workflow's jobs. Only a commit pins.
//
// Verifies: REQ-SUP-077, REQ-CI-016
func TestActionDependenciesFromGitHub(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	raw := newGitHubStub(t, map[string]string{
		"/acme/setup/v1/action.yml": "runs:\n  using: composite\n  steps:\n    - uses: actions/cache/restore@" + sha + " # v4\n" +
			"    - uses: acme/lint@v2\n    - uses: ./in-the-workspace\n",
		"/acme/lint/v2/action.yaml":           "runs:\n  using: docker\n  image: docker/Dockerfile\n",
		"/acme/lint/v2/docker/Dockerfile":     "FROM golang:1.27 AS build\nFROM gcr.io/distroless/static\nCOPY --from=build /a /a\n",
		"/acme/image/v3/tools/fmt/action.yml": "runs:\n  using: docker\n  image: docker://ghcr.io/acme/fmt:1.0\n",
		"/acme/node/v1/action.yml":            "runs:\n  using: node20\n  main: index.js\n",
		"/octo-org/shared/v5/.github/workflows/release.yml": "on: workflow_call\njobs:\n  build:\n    uses: ./.github/workflows/build.yml\n" +
			"  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n",
	})
	withRaw(t, raw.URL)
	api := newGitHubStub(t, nil)
	withGitHub(t, api.URL)
	c := NewClient(New(), t.TempDir(), time.Hour, 5*time.Second, auth.Read(t.TempDir(), environment(nil)))
	for _, test := range []struct {
		target lang.Target
		want   []string
	}{
		{lang.Target{Ecosystem: Actions, Package: "acme/setup", Version: "v1"},
			[]string{"actions:acme/lint@v2", "actions:actions/cache@" + sha + " restore pinned"}},
		{lang.Target{Ecosystem: Actions, Package: "acme/lint", Version: "v2"},
			[]string{"oci:gcr.io/distroless/static@", "oci:golang@1.27"}},
		{lang.Target{Ecosystem: Actions, Package: "acme/image", Version: "v3", Registry: "tools/fmt"},
			[]string{"oci:ghcr.io/acme/fmt@1.0"}},
		{lang.Target{Ecosystem: Actions, Package: "acme/node", Version: "v1"}, []string{}},
		{lang.Target{Ecosystem: Actions, Package: "octo-org/shared", Version: "v5", Registry: ".github/workflows/release.yml"},
			[]string{"actions:actions/checkout@v4", "actions:octo-org/shared@v5 .github/workflows/build.yml"}},
	} {
		_, lookup := ask(t, c, test.target)
		if lookup.Answer != trace.FromIndex {
			t.Errorf("%s: %+v", test.target.Package, lookup)
		}
		if got := actionNames(c.Dependencies(test.target)); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: %v, want %v", test.target.Package, got, test.want)
		}
	}
	for _, request := range raw.take() {
		if request[len(request)-4:] != " */*" {
			t.Errorf("raw request %q", request)
		}
	}
	if asked := api.take(); len(asked) != 0 {
		t.Errorf("the API was asked without a credential: %v", asked)
	}
	// A repository without the file, and an action named without a reference.
	if _, lookup := ask(t, c, lang.Target{Ecosystem: Actions, Package: "acme/missing", Version: "v1"}); lookup.Answer != trace.NoAnswer {
		t.Errorf("missing: %+v", lookup)
	}
	if _, lookup := ask(t, c, lang.Target{Ecosystem: Actions, Package: "acme/setup"}); lookup.Reason != trace.ReasonNoVersion {
		t.Errorf("no reference: %+v", lookup)
	}
}

// With a token for GitHub's API (GITHUB_TOKEN, gh's hosts.yml), files are read
// through the contents API with the raw media type. An Enterprise Server
// GITHUB_API_URL names is asked first, with its own token and not github.com's;
// an organization's own action is not named to github.com at all.
//
// Verifies: REQ-SUP-077, REQ-AUTH-036, REQ-SUP-038, REQ-CI-015
func TestActionDependenciesThroughTheContentsAPI(t *testing.T) {
	api := newGitHubStub(t, map[string]string{
		"/repos/acme/setup/contents/action.yml?ref=v1": "runs:\n  using: composite\n  steps:\n    - uses: acme/lint@v2\n",
	})
	withGitHub(t, api.URL)
	raw := newGitHubStub(t, nil)
	withRaw(t, raw.URL)
	enterprise := newGitHubStub(t, map[string]string{
		"/api/v3/repos/corp/deploy/contents/action.yml?ref=main": "runs:\n  using: composite\n  steps:\n    - uses: corp/login@v1\n",
	})
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "gh", "hosts.yml"), "github.com:\n    oauth_token: gho_file\n")
	variables := map[string]string{"GITHUB_API_URL": enterprise.URL + "/api/v3", "GITHUB_TOKEN": "runner"}
	credentials := auth.Read(home, environment(variables))
	config := NewDiscoverer(environment(variables), home).Discover(nil)
	config.Private(scope.New([]string{"actions:corp/*"}).Match)
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, credentials)

	if got := actionNames(c.Dependencies(lang.Target{Ecosystem: Actions, Package: "acme/setup", Version: "v1"})); !reflect.DeepEqual(got, []string{"actions:acme/lint@v2"}) {
		t.Errorf("acme/setup: %v", got)
	}
	want := []string{"/repos/acme/setup/contents/action.yml?ref=v1  application/vnd.github.raw+json"}
	if got := api.take(); !reflect.DeepEqual(got, want) {
		t.Errorf("API requests %v, want %v", got, want)
	}
	if got := enterprise.take(); len(got) != 2 || got[0] != "/api/v3/repos/acme/setup/contents/action.yml?ref=v1 Bearer runner application/vnd.github.raw+json" {
		t.Errorf("the instance was asked %v", got)
	}
	if got := actionNames(c.Dependencies(lang.Target{Ecosystem: Actions, Package: "corp/deploy", Version: "main"})); !reflect.DeepEqual(got, []string{"actions:corp/login@v1"}) {
		t.Errorf("corp/deploy: %v", got)
	}
	enterprise.take()
	if _, lookup := ask(t, c, lang.Target{Ecosystem: Actions, Package: "corp/missing", Version: "main"}); lookup.Answer != trace.NoAnswer {
		t.Errorf("corp/missing: %+v", lookup)
	}
	if asked := append(api.take(), raw.take()...); len(asked) != 0 {
		t.Errorf("an organization's action was named to github.com: %v", asked)
	}
}

// A GitHub API that refuses to answer (its limit without a token) is noted once.
//
// Verifies: REQ-SUP-077, REQ-TRC-017
func TestAGitHubLimitIsNoted(t *testing.T) {
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(limited.Close)
	withGitHub(t, limited.URL)
	credentials := auth.Read(t.TempDir(), environment(map[string]string{"GITHUB_TOKEN": "t"}))
	c := NewClient(New(), t.TempDir(), time.Hour, 5*time.Second, credentials)
	got := notesOf(t, c, lang.Target{Ecosystem: Actions, Package: "acme/a", Version: "v1"},
		lang.Target{Ecosystem: Actions, Package: "acme/b", Version: "v1"})
	if len(got) != 1 || !strings.Contains(got[0], "forbidden") {
		t.Errorf("notes %v", got)
	}
}
