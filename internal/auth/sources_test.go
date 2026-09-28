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
	// A registry under a path keeps its token for that path.
	if got := authorization(t, c, "https://nexus.corp/repository/npm-group/react"); got != "Bearer from-the-environment" {
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

// Cargo puts a registry's token into the Authorization header as it is written:
// bare for crates.io and most registries, with its scheme for those that document
// one (Artifactory's "Bearer <token>"). Each token goes to its own registry's index
// and no further - two registries of one host each get their own - and crates.io's
// goes to crates.io alone.
//
// Verifies: REQ-AUTH-008
func TestCargoTokens(t *testing.T) {
	config := []byte(`
[registries]
corp = { index = "sparse+https://crates.corp/index/" }
jfrog = { index = "sparse+https://acme.jfrog.io/artifactory/api/cargo/rust/index/" }
jfrog-dev = { index = "sparse+https://acme.jfrog.io/artifactory/api/cargo/rust-dev/index/" }
other = { index = "https://other.corp/index" }
mirror = { index = "sparse+https://mirror.corp/" }

[source.crates-io]
replace-with = "mirror"
`)
	credentials := []byte(`
[registry]
token = "cio_crates-io-token"

[registries.corp]
token = "corp-token"

[registries.jfrog]
token = "Bearer jfrog-token"

[registries.jfrog-dev]
token = "Bearer dev-token"
`)
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readCargo([][]byte{config, credentials}, nil, env(map[string]string{"CARGO_REGISTRIES_OTHER_TOKEN": "other-token"}))
	for raw, want := range map[string]string{
		"https://crates.corp/index/3/s/serde":                                    "corp-token",
		"https://crates.corp/api/v1/crates":                                      "",
		"https://acme.jfrog.io/artifactory/api/cargo/rust/index/3/s/serde":       "Bearer jfrog-token",
		"https://acme.jfrog.io/artifactory/api/cargo/rust-dev/index/3/s/serde":   "Bearer dev-token",
		"https://acme.jfrog.io/artifactory/api/cargo/rust-other/index/3/s/serde": "",
		"https://other.corp/index/3/s/serde":                                     "other-token",
		"https://crates.io/api/v1/crates/serde":                                  "cio_crates-io-token",
		"https://index.crates.io/se/rd/serde":                                    "",
		"https://mirror.corp/se/rd/serde":                                        "",
	} {
		if got := authorization(t, c, raw); got != want {
			t.Errorf("%s: %q, want %q", raw, got, want)
		}
	}
}

// Cargo's layers, each over the one before: config.toml, credentials.toml, then
// the environment (CARGO_REGISTRY_TOKEN for crates.io). A token is used only when
// Cargo's cargo:token provider would supply it: a registry whose
// credential-provider is a keychain or a program, or a global provider list
// without cargo:token, has its credential where depphunter does not look.
//
// Verifies: REQ-AUTH-008
func TestCargoTokenLayersAndProviders(t *testing.T) {
	config := []byte(`
[registry]
token = "config-crates-io"
global-credential-providers = ["cargo:token", "cargo:libsecret"]

[registries.a]
index = "sparse+https://a.corp/index/"
token = "config-a"
[registries.b]
index = "sparse+https://b.corp/index/"
token = "config-b"
[registries.keychain]
index = "sparse+https://keychain.corp/index/"
token = "unused"
credential-provider = "cargo:macos-keychain"
[registries.program]
index = "sparse+https://program.corp/index/"
token = "unused"
credential-provider = ["cargo:token-from-stdout", "vault", "read"]
[registries.explicit]
index = "sparse+https://explicit.corp/index/"
token = "explicit-token"
credential-provider = "cargo:token"
[registries.bad]
index = "sparse+https://bad.corp/index/"
token = "line\nbreak"
`)
	credentials := []byte("[registries.b]\ntoken = \"credentials-b\"\n")
	vars := map[string]string{"CARGO_REGISTRY_TOKEN": "env-crates-io", "CARGO_REGISTRIES_A_TOKEN": "env-a"}
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readCargo([][]byte{config, credentials}, nil, env(vars))
	for raw, want := range map[string]string{
		"https://crates.io/api/v1/me":     "env-crates-io",
		"https://a.corp/index/1/a":        "env-a",
		"https://b.corp/index/1/a":        "credentials-b",
		"https://keychain.corp/index/1/a": "",
		"https://program.corp/index/1/a":  "",
		"https://explicit.corp/index/1/a": "explicit-token",
		"https://bad.corp/index/1/a":      "",
	} {
		if got := authorization(t, c, raw); got != want {
			t.Errorf("%s: %q, want %q", raw, got, want)
		}
	}

	// A global list without cargo:token turns the plaintext tokens off, except for a
	// registry naming cargo:token itself; the variables override the files.
	vars["CARGO_REGISTRY_GLOBAL_CREDENTIAL_PROVIDERS"] = "cargo:libsecret"
	vars["CARGO_REGISTRIES_KEYCHAIN_CREDENTIAL_PROVIDER"] = "cargo:token"
	c = &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readCargo([][]byte{config, credentials}, nil, env(vars))
	for raw, want := range map[string]string{
		"https://crates.io/api/v1/me":     "",
		"https://a.corp/index/1/a":        "",
		"https://keychain.corp/index/1/a": "unused",
		"https://explicit.corp/index/1/a": "explicit-token",
	} {
		if got := authorization(t, c, raw); got != want {
			t.Errorf("global list without cargo:token: %s: %q, want %q", raw, got, want)
		}
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

// Terraform's and OpenTofu's registry tokens: credentials blocks of the CLI
// configuration (TF_CLI_CONFIG_FILE instead of ~/.terraformrc when set),
// credentials.tfrc.json from terraform login, and TF_TOKEN_<host> variables for the
// hosts named and HCP Terraform. A host block names a host without a token.
//
// Verifies: REQ-AUTH-015
func TestTerraformTokens(t *testing.T) {
	home := t.TempDir()
	os.WriteFile(filepath.Join(home, ".terraformrc"), []byte("# CLI config\ncredentials \"tf.corp.test\" {\n  token = \"rc-token\"\n}\n\nhost \"mirror.corp-x.test\" {\n  services = {}\n}\n"), 0o644)
	os.MkdirAll(filepath.Join(home, ".terraform.d"), 0o755)
	os.WriteFile(filepath.Join(home, ".terraform.d", "credentials.tfrc.json"), []byte(`{"credentials":{"login.corp.test":{"token":"login-token"}}}`), 0o644)
	c := Read(home, func(k string) string {
		return map[string]string{"TF_TOKEN_app_terraform_io": "hcp-token", "TF_TOKEN_mirror_corp__x_test": "env-token"}[k]
	})
	for host, want := range map[string]string{"tf.corp.test": "rc-token", "login.corp.test": "login-token",
		"app.terraform.io": "hcp-token", "mirror.corp-x.test": "env-token"} {
		if c.bearer[host] != want || !c.TerraformHost(host) {
			t.Errorf("%s: token %q, known %v", host, c.bearer[host], c.TerraformHost(host))
		}
	}
	if c.TerraformHost("registry.corp.test") {
		t.Error("a host nothing names is known")
	}
	cfg := filepath.Join(t.TempDir(), "cli.tfrc")
	os.WriteFile(cfg, []byte("credentials \"only.corp.test\" { token = \"cfg-token\" }\n"), 0o644)
	c = Read(home, func(k string) string { return map[string]string{"TF_CLI_CONFIG_FILE": cfg}[k] })
	if c.bearer["only.corp.test"] != "cfg-token" || c.TerraformHost("tf.corp.test") {
		t.Errorf("TF_CLI_CONFIG_FILE was not read instead of ~/.terraformrc: %v", c.bearer)
	}
}
