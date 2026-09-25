package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// noHelper stands in for a machine with no credential helper installed.
func noHelper(string) (string, error) { return "", errors.New("not found") }

// Verifies: REQ-AUTH-001, REQ-AUTH-009
func TestNpmrcCredentialForms(t *testing.T) {
	t.Setenv("NPM_TOKEN", "from-the-environment")
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readNpmrc([]byte(`
registry=https://registry.npmjs.org
//nexus.corp/repository/npm-group/:_authToken=${NPM_TOKEN}
//basic.corp/:_auth=` + base64.StdEncoding.EncodeToString([]byte("build:secret")) + `
//split.corp/:username=dev
//split.corp/:_password=` + base64.StdEncoding.EncodeToString([]byte("p4ss")) + `
//nothing.corp/:email=dev@corp
`))
	if got := c.bearer["nexus.corp"]; got != "from-the-environment" {
		t.Errorf("nexus.corp: %q - npm's ${VAR} was not resolved", got)
	}
	if got := c.basic["basic.corp"]; got != "build:secret" {
		t.Errorf("basic.corp: %q", got)
	}
	// The pair written as two fields, with the password base64 as npm writes it.
	if got := c.basic["split.corp"]; got != "dev:p4ss" {
		t.Errorf("split.corp: %q", got)
	}
	if _, ok := c.basic["nothing.corp"]; ok {
		t.Error("an email address was taken for a credential")
	}
}

// Verifies: REQ-AUTH-005
func TestDockerStoredCredentials(t *testing.T) {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readDockerConfig([]byte(`{"auths":{
		"ghcr.io":                    {"auth":"`+base64.StdEncoding.EncodeToString([]byte("user:ghp_token"))+`"},
		"https://index.docker.io/v1/": {"auth":"`+base64.StdEncoding.EncodeToString([]byte("hub:hubpass"))+`"},
		"harbor.corp:5000":           {"username":"robot","password":"r0b0t"},
		"broken.corp":                {"auth":"not-base64"}
	}}`), noHelper)

	if got := c.basic["ghcr.io"]; got != "user:ghp_token" {
		t.Errorf("ghcr.io: %q", got)
	}
	// Docker Hub is written as a v1 API URL and served from another host entirely.
	if got := c.basic["registry-1.docker.io"]; got != "hub:hubpass" {
		t.Errorf("docker hub: %q", got)
	}
	// A registry on a port is a host and a port; both belong in the key.
	if got := c.basic["harbor.corp:5000"]; got != "robot:r0b0t" {
		t.Errorf("harbor.corp:5000: %q", got)
	}
	if _, ok := c.basic["broken.corp"]; ok {
		t.Error("an entry that is not base64 became a credential")
	}
}

// A helper is a program named by a configuration file, so the name is all it may
// contribute: anything that could select a program outside PATH is refused before
// anything is executed.
//
// Verifies: REQ-AUTH-006, REQ-AUTH-007
func TestOnlyANameCanNameAHelper(t *testing.T) {
	for _, name := range []string{
		"../../../bin/evil", "/bin/sh", "evil;rm -rf /", "..", "with space", "",
		`C:\evil`, "dir/helper",
	} {
		looked := false
		runHelper(name, "registry.test", func(string) (string, error) {
			looked = true
			return "", errors.New("not found")
		})
		if looked {
			t.Errorf("%q was looked up on PATH", name)
		}
	}
	// A plain name is looked up, and the lookup is what decides.
	looked := ""
	runHelper("desktop", "registry.test", func(n string) (string, error) {
		looked = n
		return "", errors.New("not found")
	})
	if looked != "docker-credential-desktop" {
		t.Errorf("looked up %q", looked)
	}
}

// Verifies: REQ-AUTH-008
func TestCargoTokens(t *testing.T) {
	config := []byte(`
[registries]
corp = { index = "sparse+https://crates.corp/index/" }
other = { index = "https://other.corp/index" }
`)
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readCargoCredentials([]byte(`
[registry]
token = "crates-io-token"

[registries.corp]
token = "corp-token"
`), config)
	if got := c.bearer["crates.corp"]; got != "corp-token" {
		t.Errorf("crates.corp: %q - the registry's name was not matched to its index", got)
	}
	// The public registry is configured by name alone, and its index is served from
	// a different host than the registry.
	if c.bearer["crates.io"] != "crates-io-token" || c.bearer["index.crates.io"] != "crates-io-token" {
		t.Errorf("crates.io: %v", c.bearer)
	}

	// A pipeline supplies the same token through the environment, named after the
	// registry with hyphens turned into underscores.
	env := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	env.readCargoEnv(func(name string) string {
		if name == "CARGO_REGISTRIES_OTHER_TOKEN" {
			return "other-token"
		}
		return ""
	}, config)
	if got := env.bearer["other.corp"]; got != "other-token" {
		t.Errorf("other.corp: %q", got)
	}
}

// An index URL may carry its own credential, which is how a private pip or Cargo
// mirror is usually configured. It must never survive into what is recorded: the
// index a package resolves from is drawn on the map and written into every export.
//
// Verifies: REQ-AUTH-012, REQ-AUTH-013
func TestACredentialInA_URL_IsTakenOutOfIt(t *testing.T) {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	got := c.FromURL("https://deploy:s3cr3t@pypi.corp/simple", true)
	if got != "https://pypi.corp/simple" {
		t.Errorf("URL kept as %q", got)
	}
	if c.basic["pypi.corp"] != "deploy:s3cr3t" {
		t.Errorf("credential not kept: %v", c.basic)
	}

	// The repository's own configuration may not supply one. The URL is still
	// stripped - it is going on the map either way - but nothing is kept from it.
	repo := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	if got := repo.FromURL("https://someone:else@npm.corp/", false); got != "https://npm.corp/" {
		t.Errorf("URL kept as %q", got)
	}
	if len(repo.basic) != 0 {
		t.Errorf("a repository supplied a credential: %v", repo.basic)
	}

	// A URL without one is returned as it was, not reassembled.
	for _, plain := range []string{"https://registry.npmjs.org", "https://pypi.org/simple", "not a url"} {
		if got := c.FromURL(plain, true); got != plain {
			t.Errorf("FromURL(%q) = %q", plain, got)
		}
	}
}

// Read must not fail on a machine that has none of these files, which is the ordinary
// case for a repository of open-source dependencies.
func TestReadWithoutAnyOfThem(t *testing.T) {
	c := Read(t.TempDir(), func(string) string { return "" })
	req, _ := http.NewRequest(http.MethodGet, "https://registry.npmjs.org/react", nil)
	c.Apply(req)
	if got := req.Header.Get("Authorization"); got != "" {
		t.Errorf("an empty machine produced %q", got)
	}
	if Read("", nil) == nil {
		t.Error("no home directory produced no store at all")
	}
}

// A bearer token is preferred over a password for the same host, and a host and port
// are matched before the host alone: a registry on a port has its own credential.
//
// Verifies: REQ-AUTH-011
func TestApplyPrefersTheMoreSpecificCredential(t *testing.T) {
	c := &Store{
		bearer: map[string]string{"harbor.corp:5000": "port-token"},
		basic:  map[string]string{"harbor.corp": "u:p"},
	}
	req, _ := http.NewRequest(http.MethodGet, "https://harbor.corp:5000/v2/x/manifests/latest", nil)
	c.Apply(req)
	if got := req.Header.Get("Authorization"); got != "Bearer port-token" {
		t.Errorf("got %q", got)
	}
}

// A registry reached on a port has a credential of its own, and a credential written
// for the bare host still reaches it. Before this was so, a Harbor on :5000 was sent
// nothing at all.
//
// Verifies: REQ-AUTH-011
func TestARegistryOnAPortIsReached(t *testing.T) {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{
		"harbor.corp:5000": "robot:r0b0t",
		"nexus.corp":       "build:pass",
	}}
	for _, c2 := range []struct{ url, want string }{
		{"https://harbor.corp:5000/v2/", "robot:r0b0t"},
		{"https://nexus.corp:8443/repository/npm", "build:pass"},
		{"https://nexus.corp/repository/npm", "build:pass"},
		{"https://elsewhere.corp:5000/v2/", ""},
	} {
		req, _ := http.NewRequest(http.MethodGet, c2.url, nil)
		c.Apply(req)
		want := ""
		if c2.want != "" {
			want = "Basic " + base64.StdEncoding.EncodeToString([]byte(c2.want))
		}
		if got := req.Header.Get("Authorization"); got != want {
			t.Errorf("%s: %q, want %q", c2.url, got, want)
		}
	}
}

// Verifies: REQ-AUTH-005
func TestDockerConfigIsReadFromDisk(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".docker"), 0o755); err != nil {
		t.Fatal(err)
	}
	auth := base64.StdEncoding.EncodeToString([]byte("robot:secret"))
	if err := os.WriteFile(filepath.Join(home, ".docker", "config.json"),
		[]byte(`{"auths":{"harbor.corp":{"auth":"`+auth+`"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c := Read(home, func(string) string { return "" })
	req, _ := http.NewRequest(http.MethodGet, "https://harbor.corp/v2/", nil)
	c.Apply(req)
	if got := req.Header.Get("Authorization"); got != "Basic "+auth {
		t.Errorf("got %q", got)
	}
}
