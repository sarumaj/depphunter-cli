package auth

import (
	"encoding/base64"
	"path/filepath"
	"testing"
)

// Bearer is the Authorization header of a token.
func bearer(token string) string { return "Bearer " + token }

// Yarn Berry's home configuration: a scope's token goes to the scope's registry, an
// npmRegistries entry's ident to its registry (and its path only), the top-level
// token to the default registry; ${VAR} and ${VAR:-fallback} come from the
// machine's environment, and a value naming an unset variable is dropped.
//
// Verifies: REQ-AUTH-024
func TestYarnBerryCredentials(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".yarnrc.yml"), `
npmAuthToken: "${TOP_TOKEN:-top-fallback}"
npmScopes:
  acme:
    npmRegistryServer: "https://npm.acme.corp"
    npmAuthToken: "${ACME_TOKEN}"
  "@gone":
    npmRegistryServer: "https://gone.corp"
    npmAuthToken: "${UNSET_IN_THIS_TEST}"
npmRegistries:
  "https://reg.corp/npm":
    npmAuthIdent: "builder:${REG_PASS}"
  "//lab.corp/api/v4/projects/7/packages/npm":
    npmAuthToken: tok7
  "https://b64.corp":
    npmAuthIdent: `+base64.StdEncoding.EncodeToString([]byte("u:p"))+`
`)
	c := Read(home, bundlerEnv(t, map[string]string{"ACME_TOKEN": "acme-secret", "REG_PASS": "pw"}))
	for u, want := range map[string]string{
		"https://npm.acme.corp/@acme%2fui/1.0.0":                    bearer("acme-secret"),
		"https://reg.corp/npm/left-pad/1.0.0":                       basicHeader("builder:pw"),
		"https://reg.corp/elsewhere":                                "",
		"https://lab.corp/api/v4/projects/7/packages/npm/x/1.0.0":   bearer("tok7"),
		"https://lab.corp/api/v4/projects/8/packages/npm/x/1.0.0":   "",
		"https://b64.corp/x":                                        basicHeader("u:p"),
		"https://registry.yarnpkg.com/react/1.0.0":                  bearer("top-fallback"),
		"https://registry.npmjs.org/react/1.0.0":                    bearer("top-fallback"),
		"https://gone.corp/@gone%2fx/1.0.0":                         "",
		"https://unrelated.corp/api/v4/projects/7/packages/npm/x/1": "",
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
}

// YARN_NPM_REGISTRY_SERVER, YARN_NPM_AUTH_TOKEN and YARN_NPM_AUTH_IDENT replace the
// top level of the home file, and YARN_RC_FILENAME names that file.
//
// Verifies: REQ-AUTH-024, REQ-AUTH-020
func TestYarnEnvironmentCredentials(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".ci-yarnrc.yml"), "npmRegistryServer: https://file.corp\nnpmAuthToken: from-file\n")
	c := Read(home, bundlerEnv(t, map[string]string{
		"YARN_RC_FILENAME":         ".ci-yarnrc.yml",
		"YARN_NPM_REGISTRY_SERVER": "https://env.corp/npm/",
		"YARN_NPM_AUTH_TOKEN":      "from-env",
	}))
	for u, want := range map[string]string{
		"https://env.corp/npm/react/1.0.0": bearer("from-env"),
		"https://env.corp/other":           "",
		"https://file.corp/react":          "",
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
	c = Read(t.TempDir(), bundlerEnv(t, map[string]string{
		"YARN_NPM_REGISTRY_SERVER": "https://ident.corp",
		"YARN_NPM_AUTH_IDENT":      "ci:secret",
	}))
	if got := authorization(t, c, "https://ident.corp/react"); got != basicHeader("ci:secret") {
		t.Errorf("ident: %q", got)
	}
}

// Bun's global bunfig: a table's token, a URL's user:password and a URL's bare
// :token, $VAR resolved from the environment; $XDG_CONFIG_HOME's file first.
//
// Verifies: REQ-AUTH-024, REQ-AUTH-020
func TestBunCredentials(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(home, ".bunfig.toml"), "[install]\nregistry = { url = \"https://home.corp/\", token = \"home\" }\n")
	writeFile(t, filepath.Join(xdg, ".bunfig.toml"), `
[install]
registry = { url = "https://bun.corp/npm/", token = "$BUN_TOKEN" }

[install.scopes]
acme = "https://ci:${ACME_PASS}@acme.corp/"
"@lab" = "https://:labtoken@lab.corp/"
table = { url = "https://table.corp", username = "u", password = "p" }
`)
	c := Read(home, bundlerEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg, "BUN_TOKEN": "bun-secret", "ACME_PASS": "pw"}))
	for u, want := range map[string]string{
		"https://bun.corp/npm/react/1.0.0": bearer("bun-secret"),
		"https://bun.corp/other":           "",
		"https://acme.corp/@acme%2fx":      basicHeader("ci:pw"),
		"https://lab.corp/@lab%2fx":        bearer("labtoken"),
		"https://table.corp/@table%2fx":    basicHeader("u:p"),
		"https://home.corp/x":              "",
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
}

// npm keeps a token per registry path: two projects' registries on one host each
// get their own, the longest matching path wins, a host-wide key serves the rest of
// the host, and a path-scoped token reaches nothing outside its path - the link
// checker's requests included.
//
// Verifies: REQ-AUTH-025, REQ-AUTH-001
func TestNpmPathScopedTokens(t *testing.T) {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readNpmrc([]byte(`
//lab.corp/api/v4/projects/1/packages/npm/:_authToken=one
//lab.corp/api/v4/projects/2/packages/npm/:_authToken=two
//lab.corp/api/v4/:_authToken=group
//split.corp/team/:username=dev
//split.corp/team/:_password=` + base64.StdEncoding.EncodeToString([]byte("p4ss")) + `
//wide.corp/:_authToken=wide
`))
	for u, want := range map[string]string{
		"https://lab.corp/api/v4/projects/1/packages/npm/@a%2fb/1.0.0": bearer("one"),
		"https://lab.corp/api/v4/projects/2/packages/npm/@a%2fb/1.0.0": bearer("two"),
		"https://lab.corp/api/v4/projects/3/packages/npm/x":            bearer("group"),
		"https://lab.corp/wiki/Home":                                   "",
		"https://lab.corp/api/v4/projects/10/packages/npm/x":           bearer("group"),
		"https://split.corp/team/x":                                    basicHeader("dev:p4ss"),
		"https://split.corp/teams":                                     "",
		"https://wide.corp/anything":                                   bearer("wide"),
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
}

// What npm's configuration holds for a registry wins over Yarn's and Bun's for the
// same one.
//
// Verifies: REQ-AUTH-024
func TestNpmrcWinsOverYarnAndBun(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".npmrc"), "//npm.corp/:_authToken=npm\n")
	writeFile(t, filepath.Join(home, ".yarnrc.yml"), "npmRegistries:\n  https://npm.corp:\n    npmAuthToken: yarn\n  https://yarn.corp:\n    npmAuthToken: yarn\n")
	writeFile(t, filepath.Join(home, ".bunfig.toml"), "[install]\nregistry = { url = \"https://yarn.corp\", token = \"bun\" }\n")
	c := Read(home, bundlerEnv(t, nil))
	if got := authorization(t, c, "https://npm.corp/x"); got != bearer("npm") {
		t.Errorf("npm.corp: %q", got)
	}
	if got := authorization(t, c, "https://yarn.corp/x"); got != bearer("yarn") {
		t.Errorf("yarn.corp: %q", got)
	}
}
