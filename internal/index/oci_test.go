package index

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/trace"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// registry is a stub container registry over https: it serves a manifest naming a
// base image for the manifest paths it holds, the status it is told to for the
// paths named, 404 for everything else, and remembers every path asked.
type registry struct {
	*httptest.Server
	mu     sync.Mutex
	asked  []string
	status map[string]int
}

func newRegistry(t *testing.T, base string, manifests ...string) *registry {
	t.Helper()
	r := &registry{status: map[string]int{}}
	r.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.asked = append(r.asked, req.URL.Path)
		code := r.status[req.URL.Path]
		r.mu.Unlock()
		if code != 0 {
			w.WriteHeader(code)
			return
		}
		for _, m := range manifests {
			if req.URL.Path == m {
				fmt.Fprintf(w, `{"annotations":{"org.opencontainers.image.base.name":%q}}`, base)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	// A plain-http request to it (TestIdentityTokenIsExchangedAtTheRegistry) is
	// expected; the server's complaint about it is not news.
	r.Config.ErrorLog = log.New(io.Discard, "", 0)
	r.StartTLS()
	t.Cleanup(r.Close)
	return r
}

func (r *registry) paths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.asked...)
}

// host is the registry as an image reference names it.
func (r *registry) host() string { return strings.TrimPrefix(r.URL, "https://") }

// ociOrder is order for an image, as one string.
func ociOrder(c *Config, image string) string { return strings.Join(order(c, OCI, image, ""), " ") }

// tlsClient is a client whose requests trust the stub registries' certificate
// (every httptest TLS server has the same one).
func tlsClient(t *testing.T, cfg *Config, r *registry, store *auth.Store) *Client {
	t.Helper()
	c := NewClient(cfg, t.TempDir(), time.Hour, 5*time.Second, store, nil)
	c.http = r.Client()
	c.http.Timeout = 5 * time.Second
	return c
}

// A [[registry]] of registries.conf rewrites an image to its mirrors and then its
// location: the mirror is asked first, under the path its location gives, and the
// location only when the mirror does not serve the image - or fails in any way, as
// containers/image moves on from a mirror.
//
// Verifies: REQ-SUP-068
func TestRegistriesConfMirrorIsAskedFirst(t *testing.T) {
	for _, tt := range []struct {
		name         string
		mirrorStatus int
		wantMirror   bool
	}{
		{"mirror has it", 0, true},
		{"mirror 404", http.StatusNotFound, false},
		{"mirror 500", http.StatusInternalServerError, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			loc := newRegistry(t, "alpine:3", "/v2/team/app/manifests/1.0")
			mirror := newRegistry(t, "debian:12", "/v2/cache/team/app/manifests/1.0")
			mirror.status["/v2/cache/team/app/manifests/1.0"] = tt.mirrorStatus
			home := t.TempDir()
			put(t, filepath.Join(home, ".config", "containers", "registries.conf"), fmt.Sprintf(`
[[registry]]
prefix = "%[1]s/team"
location = "%[1]s/team"
[[registry.mirror]]
location = "%[2]s/cache/team"
`, loc.host(), mirror.host()))
			cfg := discoverOn(home, "linux", nil)
			image := lang.Target{Ecosystem: OCI, Package: loc.host() + "/team/app", Version: "1.0"}
			if got := ociOrder(cfg, image.Package); got != mirror.URL+"/cache/team "+loc.URL {
				t.Errorf("candidates %q", got)
			}
			// The map attributes the image to its registry, not to the mirror.
			if idx, known := cfg.For(OCI, image.Package); idx != loc.URL || !known {
				t.Errorf("For: %s %v", idx, known)
			}
			c := tlsClient(t, cfg, loc, nil)
			got, l := ask(t, c, image)
			want := "alpine"
			if tt.wantMirror {
				want = "debian"
			}
			if len(got) != 1 || got[0] != want {
				t.Fatalf("got %v (%+v), want %s", got, l, want)
			}
			if tt.wantMirror != (len(loc.paths()) == 0) {
				t.Errorf("location asked %v", loc.paths())
			}
			if len(mirror.paths()) == 0 || mirror.paths()[0] != "/v2/cache/team/app/manifests/1.0" {
				t.Errorf("mirror asked %v", mirror.paths())
			}
		})
	}
}

// A mirror that serves digests alone is not asked for a tag, and one that serves
// tags alone not for a digest.
//
// Verifies: REQ-SUP-068
func TestRegistriesConfMirrorPullMode(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "containers", "registries.conf"), `
[[registry]]
location = "quay.io"
mirror-by-digest-only = true
[[registry.mirror]]
location = "digests.corp"
[[registry.mirror]]
location = "tags.corp"
pull-from-mirror = "tag-only"
`)
	cfg := discoverOn(home, "linux", nil)
	for ref, want := range map[string]string{
		"1.0":          "https://tags.corp https://quay.io",
		"sha256:abcd":  "https://digests.corp https://quay.io",
		"":             "https://tags.corp https://quay.io",
		"sha256:other": "https://digests.corp https://quay.io",
	} {
		var got []string
		for _, k := range cfg.candidatesFor(lang.Target{Ecosystem: OCI, Package: "quay.io/org/app", Version: ref}) {
			got = append(got, k.url)
		}
		if strings.Join(got, " ") != want {
			t.Errorf("%q: %v, want %s", ref, got, want)
		}
	}
}

// A blocked registry is never asked, and the report says why.
//
// Verifies: REQ-SUP-068
func TestRegistriesConfBlockedRegistryIsNeverAsked(t *testing.T) {
	reg := newRegistry(t, "alpine:3", "/v2/team/app/manifests/1.0")
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "containers", "registries.conf"), fmt.Sprintf(`
[[registry]]
prefix = "%s"
blocked = true
[[registry.mirror]]
location = "mirror.invalid"
`, reg.host()))
	cfg := discoverOn(home, "linux", nil)
	cfg.Trust([]string{reg.URL})
	c := tlsClient(t, cfg, reg, nil)
	got, l := ask(t, c, lang.Target{Ecosystem: OCI, Package: reg.host() + "/team/app", Version: "1.0"})
	if len(got) != 0 || len(reg.paths()) != 0 {
		t.Errorf("asked a blocked registry: %v %v", got, reg.paths())
	}
	if l.Reason != trace.ReasonBlocked {
		t.Errorf("reason %q", l.Reason)
	}
	// Another image of the same host under a longer, unblocked prefix is asked.
	put(t, filepath.Join(home, ".config", "containers", "registries.conf.d", "10-open.conf"), fmt.Sprintf(`
[[registry]]
location = "%s/open"
`, reg.host()))
	cfg = discoverOn(home, "linux", nil)
	if got := ociOrder(cfg, reg.host()+"/open/app"); got != reg.URL {
		t.Errorf("unblocked prefix: %q", got)
	}
}

// registries.conf and its drop-ins are merged as containers/image merges them: a
// later file's [[registry]] replaces the one with the same prefix (the location when
// no prefix is written), the system's registries.conf.d before the user's, each by
// name; CONTAINERS_REGISTRIES_CONF names the main file, and a user registries.conf
// leaves the system's files unread.
//
// Verifies: REQ-SUP-068, REQ-SUP-064
func TestRegistriesConfDropInsMergeInOrder(t *testing.T) {
	home := t.TempDir()
	sysDir := filepath.Join(userconf.SystemRoot, "etc", "containers")
	t.Cleanup(func() { os.RemoveAll(sysDir) })
	main := filepath.Join(t.TempDir(), "main.conf")
	put(t, main, `
[[registry]]
prefix = "quay.io"
location = "quay.io"
[[registry.mirror]]
location = "main.corp"
[[registry]]
location = "ghcr.io"
[[registry.mirror]]
location = "ghcr-main.corp"
`)
	put(t, filepath.Join(sysDir, "registries.conf.d", "50-sys.conf"), `
[[registry]]
location = "quay.io"
[[registry.mirror]]
location = "sys50.corp"
`)
	put(t, filepath.Join(sysDir, "registries.conf.d", "90-sys.conf"), `
[[registry]]
prefix = "ghcr.io"
location = "ghcr.io"
[[registry.mirror]]
location = "sys90.corp"
`)
	put(t, filepath.Join(sysDir, "registries.conf.d", "ignored.txt"), `[[registry]]
location = "quay.io"
blocked = true
`)
	put(t, filepath.Join(home, ".config", "containers", "registries.conf.d", "00-user.conf"), `
[[registry]]
location = "quay.io"
[[registry.mirror]]
location = "user00.corp"
`)
	cfg := discoverOn(home, "linux", map[string]string{"CONTAINERS_REGISTRIES_CONF": main})
	// The user's 00- drop-in is read after the system's 50-: it wins for quay.io.
	if got := ociOrder(cfg, "quay.io/org/app"); got != "https://user00.corp https://quay.io" {
		t.Errorf("quay.io: %q", got)
	}
	if got := ociOrder(cfg, "ghcr.io/org/app"); got != "https://sys90.corp https://ghcr.io" {
		t.Errorf("ghcr.io: %q", got)
	}
	// Without the variable, the system registries.conf is the main file.
	put(t, filepath.Join(sysDir, "registries.conf"), `
[[registry]]
location = "docker.io"
[[registry.mirror]]
location = "hub.corp/proxy"
`)
	cfg = discoverOn(home, "linux", nil)
	if got := ociOrder(cfg, "nginx"); got != "https://hub.corp/proxy "+public[OCI] {
		t.Errorf("docker.io: %q", got)
	}
	if base, repo := cfg.ociRoute("nginx", "", "https://hub.corp/proxy"); base != "https://hub.corp" || repo != "proxy/library/nginx" {
		t.Errorf("route %s %s", base, repo)
	}
	// A user registries.conf replaces the system's, and its registries.conf.d is
	// the only one read.
	put(t, filepath.Join(home, ".config", "containers", "registries.conf"), "")
	cfg = discoverOn(home, "linux", nil)
	if got := ociOrder(cfg, "nginx"); got != public[OCI] {
		t.Errorf("user file: nginx %q", got)
	}
	if got := ociOrder(cfg, "quay.io/org/app"); got != "https://user00.corp https://quay.io" {
		t.Errorf("user file: quay.io %q", got)
	}
	if got := ociOrder(cfg, "ghcr.io/org/app"); got != "https://ghcr.io?" {
		t.Errorf("user file: ghcr.io %q", got)
	}
}

// Prefixes match whole path segments, the longest wins, and a wildcard prefix
// matches the subdomains of its host and rewrites nothing.
//
// Verifies: REQ-SUP-068
func TestRegistriesConfPrefixMatching(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "containers", "registries.conf"), `
[[registry]]
prefix = "example.com/foo"
location = "internal.corp/bar"
[[registry]]
prefix = "example.com/foo/deep"
location = "deep.corp"
[[registry]]
prefix = "*.corp.io"
[[registry.mirror]]
location = "wild.mirror"
[[registry]]
prefix = "*.bad.io"
location = "never.corp"
`)
	cfg := discoverOn(home, "linux", nil)
	for image, want := range map[string]string{
		"example.com/foo/app":      "https://internal.corp",
		"example.com/foobar/app":   "https://example.com?",
		"example.com/foo/deep/app": "https://deep.corp",
		"a.corp.io/team/app":       "https://wild.mirror https://a.corp.io",
		"corp.io/team/app":         "https://corp.io?",
		"x.bad.io/app":             "https://x.bad.io?",
	} {
		if got := ociOrder(cfg, image); got != want {
			t.Errorf("%s: %q, want %q", image, got, want)
		}
	}
	// A Docker Hub image named in full, as a base image's annotation names it, is
	// Docker Hub's.
	if got := ociOrder(cfg, "docker.io/library/debian"); got != public[OCI] {
		t.Errorf("docker.io/library/debian: %q", got)
	}
	for image, want := range map[string]string{
		"docker.io/library/debian": "registry-1.docker.io library/debian",
		"example.com/foo/app":      "internal.corp bar/app",
		"example.com/foo/deep/app": "deep.corp app",
		"a.corp.io/team/app":       "a.corp.io team/app",
	} {
		index, _ := cfg.For(OCI, image)
		base, repo := cfg.ociRoute(image, "", index)
		if got := Host(base) + " " + repo; got != want {
			t.Errorf("%s: %q, want %q", image, got, want)
		}
	}
}

// The Docker daemon's registry-mirrors serve Docker Hub alone: a Hub image is asked
// of them first, in order, and of Docker Hub when they do not have it; an image of
// any other registry never reaches them.
//
// Verifies: REQ-SUP-068
func TestDockerDaemonMirrorsServeDockerHub(t *testing.T) {
	hub := newRegistry(t, "alpine:3", "/v2/library/app/manifests/1", "/v2/org/tool/manifests/2")
	mirror := newRegistry(t, "debian:12", "/v2/org/tool/manifests/2")
	was := public[OCI]
	public[OCI] = hub.URL
	t.Cleanup(func() { public[OCI] = was })
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "docker", "daemon.json"), fmt.Sprintf(`{
		"registry-mirrors": [%q, "no-scheme.corp", "https://with.path/v2"]}`, mirror.URL+"/"))
	cfg := discoverOn(home, "linux", nil)
	if got := ociOrder(cfg, "app"); got != mirror.URL+" "+hub.URL {
		t.Errorf("hub image: %q", got)
	}
	if got := ociOrder(cfg, "ghcr.io/org/app"); got != "https://ghcr.io?" {
		t.Errorf("ghcr image: %q", got)
	}
	c := tlsClient(t, cfg, hub, nil)
	// The mirror has org/tool; library/app only Docker Hub has.
	if got, l := ask(t, c, lang.Target{Ecosystem: OCI, Package: "org/tool", Version: "2"}); len(got) != 1 || got[0] != "debian" {
		t.Errorf("org/tool: %v %+v", got, l)
	}
	if got, l := ask(t, c, lang.Target{Ecosystem: OCI, Package: "app", Version: "1"}); len(got) != 1 || got[0] != "alpine" {
		t.Errorf("app: %v %+v", got, l)
	}
	if p := mirror.paths(); len(p) != 2 || p[1] != "/v2/library/app/manifests/1" {
		t.Errorf("mirror asked %v", p)
	}
	if p := hub.paths(); len(p) != 1 || p[0] != "/v2/library/app/manifests/1" {
		t.Errorf("hub asked %v", p)
	}
	// /etc/docker/daemon.json is the rootful daemon's, read when the rootless one's
	// is absent.
	sys := filepath.Join(userconf.SystemRoot, "etc", "docker", "daemon.json")
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(sys)) })
	put(t, sys, `{"registry-mirrors": ["https://etc.mirror"]}`)
	if got := ociOrder(discoverOn(t.TempDir(), "linux", nil), "app"); got != "https://etc.mirror "+hub.URL {
		t.Errorf("/etc/docker: %q", got)
	}
}

// An identity token (`identitytoken`, as `docker login` stores an Azure Container
// Registry refresh token) is exchanged at the realm of the registry's Bearer
// challenge with the OAuth2 refresh-token grant, and the access token that comes
// back pulls the image. It goes nowhere but a realm on the registry's own host over
// https: a challenge pointing elsewhere gets the anonymous GET.
//
// Verifies: REQ-AUTH-030, REQ-SUP-027
func TestIdentityTokenIsExchangedAtTheRegistry(t *testing.T) {
	var mu sync.Mutex
	var forms []string
	realm := ""
	reg := newRegistry(t, "", "")
	reg.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			if r.Method != http.MethodPost || r.Header.Get("Authorization") != "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := r.ParseForm(); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			mu.Lock()
			forms = append(forms, r.PostForm.Encode())
			mu.Unlock()
			if r.PostForm.Get("refresh_token") != "refresh-1" || r.PostForm.Get("grant_type") != "refresh_token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"access_token":"access-1","refresh_token":"refresh-2"}`)
		case "/v2/team/app/manifests/1":
			if r.Header.Get("Authorization") != "Bearer access-1" {
				w.Header().Set("Www-Authenticate", fmt.Sprintf(
					`Bearer realm=%q,service=%q,scope="repository:team/app:pull"`, realm, "reg.test"))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"annotations":{"org.opencontainers.image.base.name":"alpine:3"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	realm = reg.URL + "/oauth2/token"
	home := t.TempDir()
	put(t, filepath.Join(home, ".docker", "config.json"), fmt.Sprintf(`{"auths":{%q:{"identitytoken":"refresh-1"}}}`, reg.host()))
	store := auth.Read(home, nil)
	if store.IdentityToken(reg.host()) != "refresh-1" {
		t.Fatal("identity token not read")
	}
	cfg := New()
	cfg.Credentials(store)
	image := lang.Target{Ecosystem: OCI, Package: reg.host() + "/team/app", Version: "1"}
	c := tlsClient(t, cfg, reg, store)
	got, l := ask(t, c, image)
	if len(got) != 1 || got[0] != "alpine" {
		t.Fatalf("got %v (%+v)", got, l)
	}
	mu.Lock()
	want := "client_id=depphunter&grant_type=refresh_token&refresh_token=refresh-1&scope=repository%3Ateam%2Fapp%3Apull&service=reg.test"
	if len(forms) != 1 || forms[0] != want {
		t.Errorf("forms %q", forms)
	}
	forms = nil
	mu.Unlock()

	// A realm on another host - or over http - is not sent the refresh token.
	other := newRegistry(t, "", "")
	var otherAsked []string
	other.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_ = r.ParseForm()
		otherAsked = append(otherAsked, r.Method+" "+r.Form.Encode())
		w.WriteHeader(http.StatusUnauthorized)
	})
	for _, elsewhere := range []string{other.URL + "/oauth2/token", strings.Replace(reg.URL, "https:", "http:", 1) + "/oauth2/token"} {
		realm = elsewhere
		c := tlsClient(t, cfg, reg, store)
		c.Dependencies(image)
		mu.Lock()
		if len(forms) != 0 {
			t.Errorf("%s: the registry's token endpoint got %q", elsewhere, forms)
		}
		for _, a := range otherAsked {
			if strings.Contains(a, "refresh") || !strings.HasPrefix(a, "GET ") {
				t.Errorf("%s: another host got %q", elsewhere, a)
			}
		}
		mu.Unlock()
	}
	if len(otherAsked) != 1 {
		t.Errorf("the other realm was asked %q, want one anonymous GET", otherAsked)
	}
}

// A realm over plain http is not sent the refresh token, even on the registry's own
// host (where the anonymous GET still goes, as it did before identity tokens).
//
// Verifies: REQ-AUTH-030
func TestIdentityTokenNeverTravelsOverHTTP(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = append(got, r.Method+" "+r.Form.Encode())
		fmt.Fprint(w, `{"token":"anonymous"}`)
	}))
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "http://")
	home := t.TempDir()
	put(t, filepath.Join(home, ".docker", "config.json"), fmt.Sprintf(`{"auths":{%q:{"identitytoken":"refresh-1"}}}`, host))
	store := auth.Read(home, nil)
	c := NewClient(New(), t.TempDir(), time.Hour, 5*time.Second, store, nil)
	token, err := c.ociToken(context.Background(), "https://"+host, "team/app",
		fmt.Sprintf(`Bearer realm="%s/token",service="reg"`, srv.URL))
	if err != nil || token != "anonymous" {
		t.Fatalf("token %q, %v", token, err)
	}
	if len(got) != 1 || strings.Contains(got[0], "refresh") || !strings.HasPrefix(got[0], "GET ") {
		t.Errorf("the realm got %q", got)
	}
}
