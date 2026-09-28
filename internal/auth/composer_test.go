package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// authorization is the Authorization header Apply puts on a GET of raw.
func authorization(t *testing.T, c *Store, raw string) string {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	c.Apply(request)
	return request.Header.Get("Authorization")
}

func basicHeader(pair string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(pair))
}

// writeFile writes data to path, creating the directories on the way.
func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Each kind of auth.json credential reaches the host it names, in the form that host
// takes, and no other host.
//
// Verifies: REQ-AUTH-016, REQ-AUTH-011
func TestComposerAuthJSONKinds(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".composer", "auth.json"), `{
		"http-basic": {"repo.packagist.com": {"username": "token", "password": "pp-secret"},
		               "satis.corp:8443": {"username": "ci", "password": "port-secret"}},
		"bearer": {"repman.corp": "bearer-secret"},
		"gitlab-token": {"gitlab.corp": "glpat-secret",
		                 "deploy.gitlab.corp": {"username": "gitlab+deploy-token-1", "token": "dt-secret"}},
		"gitlab-oauth": {"gitlab.com": "oauth-secret"},
		"github-oauth": {"github.com": "ghp_secret"},
		"bitbucket-oauth": {"bitbucket.org": {"consumer-key": "k", "consumer-secret": "s"}}
	}`)
	c := Read(home, nil)
	for _, testCase := range []struct{ url, want string }{
		{"https://repo.packagist.com/acme/packages.json", basicHeader("token:pp-secret")},
		{"https://satis.corp:8443/packages.json", basicHeader("ci:port-secret")},
		{"https://repman.corp/p2/acme/lib.json", "Bearer bearer-secret"},
		{"https://gitlab.corp/api/v4/group/7/-/packages/composer/packages.json", "Bearer glpat-secret"},
		{"https://deploy.gitlab.corp/api/v4/group/7/-/packages/composer/packages.json", basicHeader("gitlab+deploy-token-1:dt-secret")},
		{"https://gitlab.com/api/v4/group/7/-/packages/composer/packages.json", "Bearer oauth-secret"},
		{"https://github.com/acme/lib", "Bearer ghp_secret"},
		{"https://api.github.com/repos/acme/lib/zipball/v1.0.0", "Bearer ghp_secret"},
		// Not named, or named on another port: nothing.
		{"https://satis.corp/packages.json", ""},
		{"https://repo.packagist.org/p2/acme/lib.json", ""},
		{"https://bitbucket.org/acme/lib", ""},
		{"https://raw.githubusercontent.com/acme/lib/main/composer.json", ""},
		// Named, but over plain http from a link in the repository: nothing.
		{"http://repman.corp/p2/acme/lib.json", ""},
	} {
		if got := authorization(t, c, testCase.url); got != testCase.want {
			t.Errorf("%s: got %q, want %q", testCase.url, got, testCase.want)
		}
	}
}

// COMPOSER_AUTH is merged over auth.json host by host, so a pipeline can replace one
// host's credential and keep the rest; COMPOSER_HOME, when set, is the only home.
//
// Verifies: REQ-AUTH-016
func TestComposerAuthEnvironmentWins(t *testing.T) {
	home, composer := t.TempDir(), t.TempDir()
	// The home Composer would not use while COMPOSER_HOME is set.
	writeFile(t, filepath.Join(home, ".composer", "auth.json"), `{"bearer": {"ignored.corp": "x"}}`)
	writeFile(t, filepath.Join(composer, "auth.json"), `{
		"http-basic": {"satis.corp": {"username": "disk", "password": "old"},
		               "kept.corp": {"username": "disk", "password": "kept"}}
	}`)
	writeFile(t, filepath.Join(composer, "config.json"), `{"config": {"bearer": {"config.corp": "from-config"}}}`)
	c := Read(home, environment(map[string]string{
		"COMPOSER_HOME": composer,
		"COMPOSER_AUTH": `{"http-basic": {"satis.corp": {"username": "ci", "password": "new"}}}`,
	}))
	if got := authorization(t, c, "https://satis.corp/packages.json"); got != basicHeader("ci:new") {
		t.Errorf("satis.corp: %q - COMPOSER_AUTH did not win over auth.json", got)
	}
	if got := authorization(t, c, "https://kept.corp/packages.json"); got != basicHeader("disk:kept") {
		t.Errorf("kept.corp: %q - COMPOSER_AUTH replaced the whole kind", got)
	}
	if got := authorization(t, c, "https://config.corp/packages.json"); got != "Bearer from-config" {
		t.Errorf("config.corp: %q - config.json's config was not read", got)
	}
	if got := authorization(t, c, "https://ignored.corp/x"); got != "" {
		t.Errorf("~/.composer was read although COMPOSER_HOME is set: %q", got)
	}

	// Composer loads bearer after http-basic, so a host named under both sends the
	// token, wherever each came from.
	c = Read(home, environment(map[string]string{
		"COMPOSER_HOME": composer,
		"COMPOSER_AUTH": `{"bearer": {"satis.corp": "tok"}}`,
	}))
	if got := authorization(t, c, "https://satis.corp/packages.json"); got != "Bearer tok" {
		t.Errorf("satis.corp: %q", got)
	}
}

// The XDG home comes before ~/.composer, as Composer picks it.
//
// Verifies: REQ-AUTH-016
func TestComposerHomeXDG(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(xdg, "composer", "auth.json"), `{"bearer": {"xdg.corp": "a"}}`)
	writeFile(t, filepath.Join(home, ".composer", "auth.json"), `{"bearer": {"legacy.corp": "b"}}`)
	c := Read(home, environment(map[string]string{"XDG_CONFIG_HOME": xdg}))
	if got := authorization(t, c, "https://xdg.corp/x"); got != "Bearer a" {
		t.Errorf("xdg.corp: %q", got)
	}
	if got := authorization(t, c, "https://legacy.corp/x"); got != "" {
		t.Errorf("legacy.corp: %q - Composer uses one home only", got)
	}

	// Without XDG_CONFIG_HOME, ~/.config/composer.
	writeFile(t, filepath.Join(home, ".config", "composer", "auth.json"), `{"bearer": {"config.corp": "c"}}`)
	c = Read(home, nil)
	if got := authorization(t, c, "https://config.corp/x"); got != "Bearer c" {
		t.Errorf("config.corp: %q", got)
	}
}

// A broken auth.json or COMPOSER_AUTH, or an entry of the wrong shape, costs its own
// credentials and nothing else.
//
// Verifies: REQ-AUTH-016
func TestComposerMalformedAuth(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".composer", "auth.json"), `{"http-basic": {"satis.corp": `)
	writeFile(t, filepath.Join(home, ".netrc"), "machine other.corp login u password p\n")
	c := Read(home, environment(map[string]string{"COMPOSER_AUTH": `{"bearer": `}))
	if got := authorization(t, c, "https://satis.corp/x"); got != "" {
		t.Errorf("satis.corp: %q", got)
	}
	if got := authorization(t, c, "https://other.corp/x"); got == "" {
		t.Error("a broken auth.json lost the netrc credentials")
	}

	c = Read(home, environment(map[string]string{"COMPOSER_AUTH": `{
		"http-basic": {"a.corp": "not-an-object", "b.corp": {"username": "u", "password": "p"},
		               "https://user:pw@c.corp": {"username": "u", "password": "p"}},
		"bearer": {"d.corp": {"token": "x"}, "e.corp": ""},
		"gitlab-token": {"f.corp": 42},
		"github-oauth": []
	}`}))
	for host, want := range map[string]string{
		"a.corp": "", "b.corp": basicHeader("u:p"), "c.corp": "", "d.corp": "", "e.corp": "", "f.corp": "",
	} {
		if got := authorization(t, c, "https://"+host+"/x"); got != want {
			t.Errorf("%s: got %q, want %q", host, got, want)
		}
	}
}

// A repository's auth.json, and the "config" of its composer.json, is not this
// machine's: Read is given the home directory, and a checkout in the working
// directory or under the home contributes nothing.
//
// Verifies: REQ-AUTH-017
func TestComposerRepositoryAuthIgnored(t *testing.T) {
	home := t.TempDir()
	repository := filepath.Join(home, "src", "app")
	writeFile(t, filepath.Join(repository, "composer.json"), `{
		"repositories": [{"type": "composer", "url": "http://satis.corp"}],
		"config": {"bearer": {"satis.corp": "repo-config"}}
	}`)
	writeFile(t, filepath.Join(repository, "auth.json"), `{"http-basic": {"satis.corp": {"username": "repo", "password": "leak"}}}`)
	t.Chdir(repository)
	c := Read(home, environment(map[string]string{}))
	for _, raw := range []string{"https://satis.corp/packages.json", "http://satis.corp/packages.json"} {
		if got := authorization(t, c, raw); got != "" {
			t.Errorf("%s: the repository's credential was taken: %q", raw, got)
		}
	}
}

// A plain-http repository of the global config.json may be sent its credential; the
// same host from a README link over http may not unless the machine named it.
//
// Verifies: REQ-AUTH-011, REQ-AUTH-016
func TestComposerPlainHTTPOnlyWhenConfigured(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".composer", "config.json"), `{
		"config": {"secure-http": false},
		"repositories": {"legacy": {"type": "composer", "url": "http://legacy.corp"}}
	}`)
	writeFile(t, filepath.Join(home, ".composer", "auth.json"), `{
		"http-basic": {"legacy.corp": {"username": "u", "password": "p"},
		               "secure.corp": {"username": "u", "password": "p"}}
	}`)
	c := Read(home, nil)
	if got := authorization(t, c, "http://legacy.corp/packages.json"); got != basicHeader("u:p") {
		t.Errorf("legacy.corp over http: %q", got)
	}
	if got := authorization(t, c, "http://secure.corp/packages.json"); got != "" {
		t.Errorf("secure.corp over http: %q", got)
	}
}

// Apply sets the header on the first request only. When a registry redirects - a
// dist URL to a CDN, say - Go's client carries Authorization only to the same host or
// one of its subdomains, so a credential does not follow a redirect elsewhere.
//
// Verifies: REQ-AUTH-011
func TestCredentialDoesNotFollowARedirectElsewhere(t *testing.T) {
	var seen string
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
	}))
	defer cdn.Close()
	// The registry is 127.0.0.1 and the CDN "localhost": two hosts, both loopback.
	elsewhere := strings.Replace(cdn.URL, "127.0.0.1", "localhost", 1)
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("the registry got no credential")
		}
		http.Redirect(w, r, elsewhere+"/dist.zip", http.StatusFound)
	}))
	defer registry.Close()

	home := t.TempDir()
	host := strings.TrimPrefix(registry.URL, "http://")
	writeFile(t, filepath.Join(home, ".composer", "auth.json"), `{"bearer": {"`+host+`": "tok"}}`)
	c := Read(home, nil)
	request, _ := http.NewRequest(http.MethodGet, registry.URL+"/p2/acme/lib.json", nil)
	c.Apply(request)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if seen != "" {
		t.Errorf("the credential followed the redirect to another host: %q", seen)
	}
}

// environment serves a fixed environment.
func environment(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}
