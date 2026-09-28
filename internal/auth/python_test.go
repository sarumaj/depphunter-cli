package auth

import (
	"path/filepath"
	"testing"
)

// uv's UV_INDEX_<NAME>_USERNAME/_PASSWORD go to the index of that name this
// machine's uv configuration names (uv.toml or UV_INDEX), Poetry's http-basic to
// the repository of its name (auth.toml over config.toml, the environment over
// both), PDM's [pypi] and [pypi.<name>] credentials to their URLs (PDM_PYPI_*
// over [pypi]'s). Each serves its index's path, over netrc's credential for the
// host; a name no machine index carries goes nowhere.
//
// Verifies: REQ-AUTH-026, REQ-AUTH-020
func TestPythonToolCredentials(t *testing.T) {
	home, poetry := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(home, ".netrc"), "machine uv.corp login netrc password netrc\n")
	writeFile(t, filepath.Join(home, ".config", "uv", "uv.toml"),
		"[[index]]\nname = \"corp\"\nurl = \"https://uv.corp/api/pypi/simple\"\n\n[[index]]\nname = \"root\"\nurl = \"https://root.corp/simple/\"\n")
	writeFile(t, filepath.Join(poetry, "config.toml"),
		"[repositories.poetry-corp]\nurl = \"https://poetry.corp/repo/simple\"\n\n[http-basic.poetry-corp]\nusername = \"file\"\npassword = \"file\"\n")
	writeFile(t, filepath.Join(poetry, "auth.toml"), "[http-basic.poetry-corp]\nusername = \"auth\"\npassword = \"auth\"\n")
	writeFile(t, filepath.Join(home, ".config", "pdm", "config.toml"),
		"[pypi]\nurl = \"https://pdm.corp/simple\"\nusername = \"pdm\"\npassword = \"file\"\n\n[pypi.extra]\nurl = \"https://extra.corp/x/simple\"\nusername = \"e\"\npassword = \"e\"\n")
	c := onMachine(t, home, "linux", map[string]string{
		"POETRY_CONFIG_DIR":                      poetry,
		"POETRY_HTTP_BASIC_POETRY_CORP_PASSWORD": "env",
		"UV_INDEX":                               "envidx=https://envidx.corp/simple",
		"UV_INDEX_CORP_USERNAME":                 "uv",
		"UV_INDEX_CORP_PASSWORD":                 "uv-secret",
		"UV_INDEX_ROOT_PASSWORD":                 "root-token",
		"UV_INDEX_ENVIDX_USERNAME":               "ev",
		"UV_INDEX_ENVIDX_PASSWORD":               "ev",
		"UV_INDEX_NOWHERE_PASSWORD":              "lost",
		"PDM_PYPI_PASSWORD":                      "env",
	})
	for u, want := range map[string]string{
		"https://uv.corp/api/pypi/pypi/x/json":    basicHeader("uv:uv-secret"),
		"https://uv.corp/api/pypi/simple/x/":      basicHeader("uv:uv-secret"),
		"https://uv.corp/elsewhere":               basicHeader("netrc:netrc"),
		"https://root.corp/pypi/x/json":           basicHeader(":root-token"),
		"https://envidx.corp/pypi/x/json":         basicHeader("ev:ev"),
		"https://poetry.corp/repo/pypi/x/json":    basicHeader("auth:env"),
		"https://poetry.corp/other":               "",
		"https://pdm.corp/pypi/x/json":            basicHeader("pdm:env"),
		"https://extra.corp/x/pypi/y/json":        basicHeader("e:e"),
		"https://extra.corp/y/pypi/y/json":        "",
		"https://nowhere.corp/pypi/x/json":        "",
		"https://uv.corp/api/pypi-other/x/simple": basicHeader("netrc:netrc"),
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
}

// UV_NO_CONFIG leaves the uv.toml files unread, and with them the credentials of
// the indexes only they name.
//
// Verifies: REQ-AUTH-026
func TestUVNoConfigReadsNoFile(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".config", "uv", "uv.toml"), "[[index]]\nname = \"corp\"\nurl = \"https://uv.corp/simple\"\n")
	c := onMachine(t, home, "linux", map[string]string{"UV_NO_CONFIG": "1", "UV_INDEX_CORP_PASSWORD": "p"})
	if got := authorization(t, c, "https://uv.corp/pypi/x/json"); got != "" {
		t.Errorf("sent %q", got)
	}
}

// A lent Python index credential serves the index's path and never replaces what
// this machine holds there.
//
// Verifies: REQ-AUTH-023, REQ-AUTH-026
func TestLendIndex(t *testing.T) {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{"mine.corp": "me:mine"}}
	c.LendIndex("https://mine.corp/simple", "ci", "lent")
	c.LendIndex("https://gitlab.corp/api/v4/projects/7/packages/pypi/simple", "__token__", "tok")
	c.LendIndex("https://empty.corp/simple", "", "")
	for u, want := range map[string]string{
		"https://mine.corp/pypi/x/json":                                   basicHeader("me:mine"),
		"https://gitlab.corp/api/v4/projects/7/packages/pypi/pypi/x/json": basicHeader("__token__:tok"),
		"https://gitlab.corp/api/v4/projects/8/packages/pypi/pypi/x/json": "",
		"https://empty.corp/pypi/x/json":                                  "",
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
	var nilStore *Store
	nilStore.LendIndex("https://x.corp/simple", "a", "b")
}

// A Python index credential also serves the files and PEP 658 metadata the index
// keeps below the same path (devpi's /+simple is its simple suffix), and nothing
// outside it: a file on another path or host gets what is filed for that one.
//
// Verifies: REQ-AUTH-026, REQ-SUP-067
func TestPythonCredentialCoversTheIndexFiles(t *testing.T) {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.LendIndex("https://gitlab.corp/api/v4/projects/7/packages/pypi/simple", "__token__", "tok")
	c.LendIndex("https://devpi.corp/team/prod/+simple/", "dev", "pi")
	c.LendIndex("https://nexus.corp/repository/pypi-internal/simple/", "nx", "pw")
	for u, want := range map[string]string{
		"https://gitlab.corp/api/v4/projects/7/packages/pypi/simple/lib/":                                  basicHeader("__token__:tok"),
		"https://gitlab.corp/api/v4/projects/7/packages/pypi/files/0a1b/lib-1.0-py3-none-any.whl.metadata": basicHeader("__token__:tok"),
		"https://devpi.corp/team/prod/+f/0a1/b2c/lib-1.0-py3-none-any.whl.metadata":                        basicHeader("dev:pi"),
		"https://devpi.corp/team/other/+f/0a1/b2c/lib-1.0-py3-none-any.whl.metadata":                       "",
		"https://nexus.corp/repository/pypi-internal/packages/lib/1.0/lib-1.0.tar.gz.metadata":             basicHeader("nx:pw"),
		"https://files.nexus.corp/repository/pypi-internal/packages/lib-1.0.tar.gz.metadata":               "",
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
}
