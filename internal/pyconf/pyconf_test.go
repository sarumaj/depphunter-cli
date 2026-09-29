package pyconf

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// show prints indexes as "name kind url", one per line, for comparison.
func show(indexes []Index) string {
	var out []string
	for _, i := range indexes {
		kind := [...]string{"extra", "default", "primary", "explicit", "supplemental"}[i.Kind]
		line := strings.TrimSpace(fmt.Sprintf("%s %s %s", i.Name, kind, i.URL))
		if len(i.Include) > 0 {
			line += " include=" + strings.Join(i.Include, ",")
		}
		if i.Username != "" || i.Password != "" {
			line += " cred=" + i.Username + ":" + i.Password
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func pins(s Settings) string {
	var out []string
	for packageName, index := range s.Pins {
		out = append(out, packageName+"->"+index)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

func check(t *testing.T, what, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s:\n got %s\nwant %s", what, got, want)
	}
}

// uv's [[index]] entries keep their order and kind, the legacy index-url and
// extra-index-url are read, and [tool.uv.sources] pins by index name (a table or
// a list of them); uv.toml has the same keys at its top level.
//
// Verifies: REQ-SUP-066
func TestUV(t *testing.T) {
	py := `
[tool.uv]
index-url = "https://legacy.corp/simple"
extra-index-url = ["https://legacy-extra.corp/simple"]

[[tool.uv.index]]
name = "a"
url = "https://a.corp/simple"

[[tool.uv.index]]
name = "torch"
url = "https://download.pytorch.org/whl/cpu"
explicit = true

[[tool.uv.index]]
name = "main"
url = "https://main.corp/simple"
default = true

[tool.uv.sources]
torch = { index = "torch" }
tv = [{ index = "torch", marker = "sys_platform == 'linux'" }, { index = "a" }]
local = { path = "../local" }
`
	s, ok := UV([]byte(py), true)
	if !ok {
		t.Fatal("not read")
	}
	check(t, "pyproject indexes", show(s.Indexes), "default https://legacy.corp/simple\na extra https://a.corp/simple\n"+
		"torch explicit https://download.pytorch.org/whl/cpu\nmain default https://main.corp/simple\nextra https://legacy-extra.corp/simple")
	check(t, "pins", pins(s), "torch->torch tv->torch")

	s, _ = UV([]byte("[[index]]\nname = \"corp\"\nurl = \"https://uv.corp/simple\"\ndefault = true\n\n[sources]\nx = { index = \"corp\" }\n"), false)
	check(t, "uv.toml", show(s.Indexes), "corp default https://uv.corp/simple")
	check(t, "uv.toml has no sources", pins(s), "")
	if _, ok := UV([]byte("[[index]\n"), false); ok {
		t.Error("broken TOML read")
	}
}

// UV_DEFAULT_INDEX (or UV_INDEX_URL) comes first, then UV_INDEX and
// UV_EXTRA_INDEX_URL, space separated; name=url names an entry, and a URL's own
// query string is not taken for a name.
//
// Verifies: REQ-SUP-066
func TestUVEnvironment(t *testing.T) {
	variables := map[string]string{
		"UV_DEFAULT_INDEX":   "main=https://main.corp/simple",
		"UV_INDEX_URL":       "https://legacy.corp/simple",
		"UV_INDEX":           "corp=https://corp.corp/simple  https://q.corp/simple?x=y",
		"UV_EXTRA_INDEX_URL": "https://extra.corp/simple",
	}
	got := show(UVEnvironment(func(k string) string { return variables[k] }))
	check(t, "env", got, "main default https://main.corp/simple\ndefault https://legacy.corp/simple\n"+
		"corp extra https://corp.corp/simple\nextra https://q.corp/simple?x=y\nextra https://extra.corp/simple")
	user, pass := UVCredential(func(k string) string {
		return map[string]string{"UV_INDEX_MY_CORP_2_USERNAME": "u", "UV_INDEX_MY_CORP_2_PASSWORD": "p"}[k]
	}, "my-corp.2")
	if user != "u" || pass != "p" {
		t.Errorf("UV_INDEX_<NAME>_*: %q %q", user, pass)
	}
	if got := EnvironmentName("pytorch-cu121.x"); got != "PYTORCH_CU121_X" {
		t.Errorf("EnvName: %s", got)
	}
}

// Poetry's sources come in the order Poetry asks them: default first, then the
// primary (and explicit) ones in the order written, then the legacy secondary and
// the supplemental ones; a source with no priority is primary, PyPI without a URL
// is PyPI; dependencies of every group pin by source. A primary source, or PyPI
// declared, drops the implicit PyPI.
//
// Verifies: REQ-SUP-066
func TestPoetry(t *testing.T) {
	s, ok := Poetry([]byte(`
[tool.poetry.dependencies]
acme = { version = "^1", source = "private" }
requests = "*"

[tool.poetry.group.dev.dependencies]
tool = { version = "*", source = "second" }

[[tool.poetry.source]]
name = "first"
url = "https://first.corp/simple"

[[tool.poetry.source]]
name = "PyPI"
priority = "primary"

[[tool.poetry.source]]
name = "supp"
url = "https://supp.corp/simple"
priority = "supplemental"

[[tool.poetry.source]]
name = "legacy"
url = "https://legacy.corp/simple"
secondary = true

[[tool.poetry.source]]
name = "private"
url = "https://private.corp/simple"
priority = "explicit"

[[tool.poetry.source]]
name = "second"
url = "https://second.corp/simple"

[[tool.poetry.source]]
name = "old-default"
url = "https://default.corp/simple"
default = true
`))
	if !ok {
		t.Fatal("not read")
	}
	check(t, "order", show(s.Indexes), "old-default default https://default.corp/simple\nfirst primary https://first.corp/simple\n"+
		"PyPI primary\nprivate explicit https://private.corp/simple\nsecond primary https://second.corp/simple\n"+
		"legacy supplemental https://legacy.corp/simple\nsupp supplemental https://supp.corp/simple")
	check(t, "pins", pins(s), "acme->private tool->second")
	if !PoetryPyPI(s.Indexes[2]) || PoetryPyPI(s.Indexes[1]) {
		t.Error("PoetryPyPI")
	}
	if !s.NoImplicitPyPI {
		t.Error("primary sources keep the implicit PyPI")
	}
	for body, want := range map[string]bool{
		"[[tool.poetry.source]]\nname = \"supp\"\nurl = \"https://supp.corp/simple\"\npriority = \"supplemental\"\n": false,
		"[[tool.poetry.source]]\nname = \"x\"\nurl = \"https://x.corp/simple\"\npriority = \"explicit\"\n":           false,
		"[[tool.poetry.source]]\nname = \"pypi\"\npriority = \"explicit\"\n":                                         true,
		"[[tool.poetry.source]]\nname = \"x\"\nurl = \"https://x.corp/simple\"\n":                                    true,
	} {
		if s, _ := Poetry([]byte(body)); s.NoImplicitPyPI != want {
			t.Errorf("%q: NoImplicitPyPI %v", body, s.NoImplicitPyPI)
		}
	}
}

// Pipfile: the first source replaces PyPI, the others are extra; a package of any
// category pins with index; Pipfile.lock's _meta.sources and "index" read alike.
//
// Verifies: REQ-SUP-066
func TestPipfile(t *testing.T) {
	s, ok := Pipfile([]byte(`
[[source]]
url = "https://${USER}:${PASS}@pypi.corp/simple"
verify_ssl = true
name = "corp"

[[source]]
url = "https://pypi.org/simple"
name = "pypi"

[packages]
requests = "*"
acme = { version = "*", index = "corp" }

[dev-packages]
pytest = { version = "*", index = "pypi" }

[requires]
python_version = "3.12"
`))
	if !ok {
		t.Fatal("not read")
	}
	check(t, "sources", show(s.Indexes), "corp default https://${USER}:${PASS}@pypi.corp/simple\npypi extra https://pypi.org/simple")
	check(t, "pins", pins(s), "acme->corp pytest->pypi")

	s, ok = PipfileLock([]byte(`{"_meta": {"sources": [{"name": "corp", "url": "https://pypi.corp/simple", "verify_ssl": true}]},
"default": {"acme": {"index": "corp", "version": "==1.0"}, "requests": {"version": "==2.31.0"}},
"develop": {}}`))
	if !ok {
		t.Fatal("lock not read")
	}
	check(t, "lock sources", show(s.Indexes), "corp default https://pypi.corp/simple")
	check(t, "lock pins", pins(s), "acme->corp")
	if _, ok := PipfileLock([]byte(`{"default": {}}`)); ok {
		t.Error("a lock without _meta read")
	}
}

// PDM: a source named pypi replaces PyPI, the others are extra with their
// include_packages; a find_links source is no index. Its config files name [pypi]
// and [pypi.<name>], with credentials.
//
// Verifies: REQ-SUP-066
func TestPDM(t *testing.T) {
	s, _ := PDM([]byte(`
[[tool.pdm.source]]
name = "private"
url = "https://${PDM_USER}:${PDM_PASS}@pdm.corp/simple"
include_packages = ["acme", "acme-*"]
exclude_packages = ["other"]

[[tool.pdm.source]]
name = "pypi"
url = "https://mirror.corp/simple"

[[tool.pdm.source]]
name = "links"
url = "https://links.corp/"
type = "find_links"
`))
	check(t, "sources", show(s.Indexes), "private extra https://${PDM_USER}:${PDM_PASS}@pdm.corp/simple include=acme,acme-*\n"+
		"pypi default https://mirror.corp/simple")

	s, _ = PDMConfig([]byte(`
[pypi]
url = "https://pdm-mirror.corp/simple"
username = "mu"
password = "mp"

[pypi.extra]
url = "https://extra.corp/simple"
username = "eu"
password = "ep"

[pypi.links]
url = "https://links.corp/"
type = "find_links"
`))
	check(t, "config", show(s.Indexes), "pypi default https://pdm-mirror.corp/simple cred=mu:mp\nextra extra https://extra.corp/simple cred=eu:ep")
}

// PDM's global config.toml is found through userconf; PDM_PYPI_URL and its
// credential variables replace [pypi]'s, or make one.
//
// Verifies: REQ-SUP-066, REQ-AUTH-026
func TestPDMMachine(t *testing.T) {
	home := t.TempDir()
	file := filepath.Join(home, ".config", "pdm", "config.toml")
	write(t, file, "[pypi]\nurl = \"https://file.corp/simple\"\nusername = \"fu\"\npassword = \"fp\"\n")
	m := userconf.Machine{Home: home, GOOS: "linux", Environment: func(k string) string {
		return map[string]string{"PDM_PYPI_URL": "https://env.corp/simple", "PDM_PYPI_PASSWORD": "ep"}[k]
	}}
	check(t, "env over file", show(PDMMachine(m).Indexes), "pypi default https://env.corp/simple cred=fu:ep")
	os.Remove(file)
	check(t, "env alone", show(PDMMachine(m).Indexes), "pypi default https://env.corp/simple cred=:ep")
}

// Poetry's config.toml names repositories and http-basic credentials, auth.toml
// credentials over them, POETRY_REPOSITORIES_<NAME>_URL and
// POETRY_HTTP_BASIC_<NAME>_* over both; names compare as EnvironmentName spells them.
//
// Verifies: REQ-AUTH-026, REQ-SUP-064
func TestPoetryMachine(t *testing.T) {
	directory := t.TempDir()
	write(t, filepath.Join(directory, "config.toml"), "[repositories.my-corp]\nurl = \"https://file.corp/simple\"\n\n[http-basic.my-corp]\nusername = \"cu\"\npassword = \"cp\"\n")
	write(t, filepath.Join(directory, "auth.toml"), "[http-basic.my-corp]\nusername = \"au\"\npassword = \"ap\"\n\n[http-basic.other]\nusername = \"ou\"\npassword = \"op\"\n")
	variables := map[string]string{
		"POETRY_CONFIG_DIR":                directory,
		"POETRY_REPOSITORIES_ENVREPO_URL":  "https://env.corp/simple",
		"POETRY_HTTP_BASIC_OTHER_PASSWORD": "env-op",
		"POETRY_HTTP_BASIC_LATE_USERNAME":  "lu",
		"POETRY_HTTP_BASIC_LATE_PASSWORD":  "lp",
	}
	m := userconf.Machine{GOOS: "linux", Environment: func(k string) string { return variables[k] }, Environ: func() []string {
		var out []string
		for k, v := range variables {
			out = append(out, k+"="+v)
		}
		return out
	}}
	repositories, credentials := PoetryMachine(m)
	check(t, "repos", fmt.Sprint(repositories), "map[ENVREPO:https://env.corp/simple MY_CORP:https://file.corp/simple]")
	check(t, "creds", fmt.Sprint(credentials), "map[MY_CORP:{au ap} OTHER:{ou env-op}]")
	check(t, "a name only the environment knows", fmt.Sprint(PoetryCredential(m, credentials, "late")), "{lu lp}")
	check(t, "by source name", fmt.Sprint(PoetryCredential(m, credentials, "my.corp")), "{au ap}")
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
