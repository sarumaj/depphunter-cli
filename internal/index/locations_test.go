package index

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// TestMain keeps the system-wide configuration of the machine running the tests
// (/etc/pip.conf and the like) out of every test here, and pins the platform of
// every Discoverer and credential store the tests make to Linux: their fixtures sit
// where the tools keep their files there (~/.config/...), not in macOS's
// ~/Library/Application Support or Windows's %APPDATA%. The tests of another
// platform's locations pick it through discoverOn.
func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "depphunter-system")
	if err != nil {
		panic(err)
	}
	userconf.SystemRoot = directory
	userconf.Platform = "linux"
	code := m.Run()
	os.RemoveAll(directory)
	os.Exit(code)
}

// discoverOn reads the machine configuration of a machine with home, the environment
// vars (listed and answered) and the platform goos.
func discoverOn(home, goos string, variables map[string]string) *Config {
	c := New()
	c.machine(userconf.Machine{Home: home, GOOS: goos, Environment: environment(variables), Environ: func() []string {
		var out []string
		for k, v := range variables {
			out = append(out, k+"="+v)
		}
		sort.Strings(out)
		return out
	}})
	return c
}

func put(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

// CARGO_HOME moves Cargo's config.toml; CARGO_REGISTRIES_<NAME>_INDEX defines a
// registry (serving the crates that name it with "-" or "_") or overrides one.
//
// Verifies: REQ-SUP-064
func TestDiscoverCargoHomeAndEnvironmentRegistries(t *testing.T) {
	home, cargo := t.TempDir(), t.TempDir()
	put(t, filepath.Join(home, ".cargo", "config.toml"), "[registries]\nhome = { index = \"sparse+https://home.corp/index/\" }\n")
	put(t, filepath.Join(cargo, "config.toml"), `[source.crates-io]
replace-with = "mirror"
[registries]
corp = { index = "sparse+https://crates.corp/index/" }
moved = { index = "sparse+https://old.corp/index/" }
`)
	c := discoverOn(home, "linux", map[string]string{
		"CARGO_HOME":                    cargo,
		"CARGO_REGISTRIES_MY_REG_INDEX": "sparse+https://env.corp/index/",
		"CARGO_REGISTRIES_MOVED_INDEX":  "sparse+https://new.corp/index/",
		"CARGO_REGISTRIES_MIRROR_INDEX": "sparse+https://mirror.corp/index/",
	})
	for _, testCase := range []struct{ registry, want string }{
		{"corp", "sparse+https://crates.corp/index"},
		{"my-reg", "sparse+https://env.corp/index"},
		{"my_reg", "sparse+https://env.corp/index"},
		{"moved", "sparse+https://new.corp/index"},
		{"", "sparse+https://mirror.corp/index"}, // replace-with ends at an env registry
	} {
		if got := order(c, Cargo, "lib", testCase.registry); strings.Join(got, " ") != testCase.want {
			t.Errorf("registry %q: %v, want %s", testCase.registry, got, testCase.want)
		}
	}
	if got := order(c, Cargo, "lib", "home"); len(got) != 0 {
		t.Errorf("~/.cargo was read beside CARGO_HOME: %v", got)
	}
}

// npm's registry comes from npm_config_* in any case (lower case winning), then the
// user's npmrc (NPM_CONFIG_USERCONFIG replacing ~/.npmrc), then the global npmrc.
//
// Verifies: REQ-SUP-064
func TestDiscoverNpmLocations(t *testing.T) {
	home, directory := t.TempDir(), t.TempDir()
	put(t, filepath.Join(home, ".npmrc"), "registry=https://home.corp/npm\n")
	user, global := filepath.Join(directory, "user-npmrc"), filepath.Join(directory, "etc", "npmrc")
	put(t, user, "registry=https://user.corp/npm\n")
	put(t, global, "registry=https://global.corp/npm\n@acme:registry=https://acme.global.corp/npm\n@beta:registry=https://beta.global.corp/npm\n")
	variables := map[string]string{
		"NPM_CONFIG_USERCONFIG":     user,
		"npm_config_prefix":         directory,
		"npm_config_@beta:registry": "https://beta.env.corp/npm",
	}
	c := discoverOn(home, "linux", variables)
	for packageName, want := range map[string]string{
		"lodash":     "https://user.corp/npm",
		"@acme/tool": "https://acme.global.corp/npm",
		"@beta/tool": "https://beta.env.corp/npm",
	} {
		if index, known := c.For(NPM, packageName); index != want || !known {
			t.Errorf("%s: %s (known %v), want %s", packageName, index, known, want)
		}
	}
	variables["NPM_CONFIG_REGISTRY"] = "https://upper.corp/npm"
	variables["npm_config_registry"] = "https://lower.corp/npm"
	if index, _ := discoverOn(home, "linux", variables).For(NPM, "lodash"); index != "https://lower.corp/npm" {
		t.Errorf("environment: %s, want the lower-case variable's", index)
	}
	if index, _ := discoverOn(home, "linux", nil).For(NPM, "lodash"); index != "https://home.corp/npm" {
		t.Errorf("default: %s, want ~/.npmrc's", index)
	}
}

// pip's layers: the site-wide files, then the user's, then PIP_CONFIG_FILE (which
// hides the user's when it exists), then PIP_INDEX_URL and PIP_EXTRA_INDEX_URL; each
// setting replaced by a later layer, extra-index-url included. PIP_CONFIG_FILE set
// to the null device leaves only the environment.
//
// Verifies: REQ-SUP-015, REQ-SUP-064
func TestDiscoverPipLayers(t *testing.T) {
	saved := userconf.SystemRoot
	userconf.SystemRoot = t.TempDir()
	t.Cleanup(func() { userconf.SystemRoot = saved })
	home, xdg := t.TempDir(), t.TempDir()
	put(t, filepath.Join(userconf.SystemRoot, "etc", "pip.conf"), "[global]\nindex-url = https://site.corp/simple\nextra-index-url =\n  https://site-extra.corp/simple\n")
	put(t, filepath.Join(userconf.SystemRoot, "etc", "xdg", "pip", "pip.conf"), "[global]\nindex-url = https://xdg-site.corp/simple\n")
	put(t, filepath.Join(xdg, "pip", "pip.conf"), "[global]\nindex-url = https://user.corp/simple\n")
	put(t, filepath.Join(home, ".pip", "pip.conf"), "[global]\nindex-url = https://legacy.corp/simple\nextra-index-url = https://legacy-extra.corp/simple\n")
	file := filepath.Join(t.TempDir(), "pip.conf")
	put(t, file, "[install]\nextra-index-url = https://file-extra.corp/simple\n")

	for _, testCase := range []struct {
		name      string
		variables map[string]string
		want      string
	}{
		{"site < legacy < user", map[string]string{"XDG_CONFIG_HOME": xdg},
			"https://legacy-extra.corp/simple https://user.corp/simple"},
		{"PIP_CONFIG_FILE hides the user's", map[string]string{"XDG_CONFIG_HOME": xdg, "PIP_CONFIG_FILE": file},
			"https://file-extra.corp/simple https://site.corp/simple"},
		{"environment over every file", map[string]string{"XDG_CONFIG_HOME": xdg, "PIP_CONFIG_FILE": file, "PIP_INDEX_URL": "https://env.corp/simple"},
			"https://file-extra.corp/simple https://env.corp/simple"},
		{"null device", map[string]string{"XDG_CONFIG_HOME": xdg, "PIP_CONFIG_FILE": "/dev/null", "PIP_EXTRA_INDEX_URL": "https://env-extra.corp/simple"},
			"https://env-extra.corp/simple https://pypi.org/simple"},
	} {
		c := discoverOn(home, "linux", testCase.variables)
		if got := strings.Join(order(c, PyPI, "requests", ""), " "); got != testCase.want {
			t.Errorf("%s:\n got %s\nwant %s", testCase.name, got, testCase.want)
		}
	}

	// Windows: %ProgramData%\pip\pip.ini, then %APPDATA%\pip\pip.ini.
	programData, appData := t.TempDir(), t.TempDir()
	put(t, filepath.Join(programData, "pip", "pip.ini"), "[global]\nindex-url = https://programdata.corp/simple\nextra-index-url = https://pd-extra.corp/simple\n")
	put(t, filepath.Join(appData, "pip", "pip.ini"), "[global]\nindex-url = https://appdata.corp/simple\n")
	c := discoverOn(home, "windows", map[string]string{"ProgramData": programData, "APPDATA": appData})
	if got := strings.Join(order(c, PyPI, "requests", ""), " "); got != "https://pd-extra.corp/simple https://appdata.corp/simple" {
		t.Errorf("windows: %s", got)
	}
}

// GOPROXY comes from the go env file (GOENV, else go/env in the user config
// directory) when the environment does not set it.
//
// Verifies: REQ-SUP-064
func TestDiscoverGoproxyFromTheGoEnvironmentFile(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "go", "env"), "GOPROXY=https://goproxy.corp,direct\n")
	if got := order(discoverOn(home, "linux", nil), Go, "corp.example/lib", ""); strings.Join(got, " ") != "https://goproxy.corp" {
		t.Errorf("go env file: %v", got)
	}
	if got := order(discoverOn(home, "linux", map[string]string{"GOPROXY": "https://env.proxy"}), Go, "corp.example/lib", ""); strings.Join(got, " ") != "https://env.proxy" {
		t.Errorf("environment: %v", got)
	}
	if got := order(discoverOn(home, "linux", map[string]string{"GOENV": "off"}), Go, "corp.example/lib", ""); strings.Join(got, " ") != "https://proxy.golang.org" {
		t.Errorf("GOENV=off: %v", got)
	}
}

// On Windows NuGet's user configuration is %APPDATA%\NuGet\NuGet.Config, and
// Composer's home %APPDATA%\Composer.
//
// Verifies: REQ-SUP-064
func TestDiscoverWindowsAppData(t *testing.T) {
	home, appData := t.TempDir(), t.TempDir()
	put(t, filepath.Join(appData, "NuGet", "NuGet.Config"), `<configuration><packageSources><add key="corp" value="https://nuget.corp/v3/index.json" /></packageSources></configuration>`)
	put(t, filepath.Join(appData, "Composer", "config.json"), `{"repositories": [{"type": "composer", "url": "https://packagist.corp"}]}`)
	put(t, filepath.Join(home, ".composer", "config.json"), `{"repositories": [{"type": "composer", "url": "https://home.corp"}]}`)
	c := discoverOn(home, "windows", map[string]string{"APPDATA": appData})
	if got := order(c, NuGet, "Acme.Tools", ""); strings.Join(got, " ") != "https://nuget.corp/v3/index.json https://api.nuget.org/v3/index.json" {
		t.Errorf("NuGet: %v", got)
	}
	if got := order(c, Composer, "acme/lib", ""); strings.Join(got, " ") != "https://packagist.corp https://repo.packagist.org" {
		t.Errorf("Composer: %v", got)
	}
}

// Index discovery takes Composer's config.json from the same single home the
// credentials come from: the XDG one when it exists, and then not ~/.composer.
//
// Verifies: REQ-SUP-064
func TestDiscoverComposerSingleHome(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".config", "composer", "config.json"), `{"repositories": [{"type": "composer", "url": "https://xdg.corp"}]}`)
	put(t, filepath.Join(home, ".composer", "config.json"), `{"repositories": [{"type": "composer", "url": "https://legacy.corp"}]}`)
	got := strings.Join(order(discoverOn(home, "linux", nil), Composer, "acme/lib", ""), " ")
	if strings.Contains(got, "legacy.corp") || !strings.Contains(got, "https://xdg.corp") {
		t.Errorf("asked %s, want the XDG home's repository and not ~/.composer's", got)
	}
}

// A relative XDG_CONFIG_HOME names no directory: taken from the working directory,
// usually the repository analyzed, it would make the repository's files this
// machine's configuration and credentials. The same files under an absolute
// XDG_CONFIG_HOME are read, so the test does not pass for another reason.
//
// Verifies: REQ-SUP-064, REQ-AUTH-020
func TestRelativeXDGConfigHomeReadsNothingFromTheRepository(t *testing.T) {
	home, repository := t.TempDir(), t.TempDir()
	put(t, filepath.Join(repository, ".config", "pip", "pip.conf"), "[global]\nindex-url = https://repository.corp/simple\n")
	credential := base64.StdEncoding.EncodeToString([]byte("repository:secret"))
	put(t, filepath.Join(repository, ".config", "containers", "auth.json"), `{"auths":{"registry.corp":{"auth":"`+credential+`"}}}`)
	t.Chdir(repository)

	for _, testCase := range []struct {
		xdgConfigHome, index, authorization string
	}{
		{".config", "https://pypi.org/simple", ""},
		{filepath.Join(repository, ".config"), "https://repository.corp/simple", "Basic " + credential},
	} {
		variables := map[string]string{"XDG_CONFIG_HOME": testCase.xdgConfigHome}
		if got := strings.Join(order(Discover(nil, environment(variables), home), PyPI, "requests", ""), " "); got != testCase.index {
			t.Errorf("XDG_CONFIG_HOME=%s: pip index %s, want %s", testCase.xdgConfigHome, got, testCase.index)
		}
		request, err := http.NewRequest(http.MethodGet, "https://registry.corp/v2/", nil)
		if err != nil {
			t.Fatal(err)
		}
		auth.Read(home, environment(variables)).Apply(request)
		if got := request.Header.Get("Authorization"); got != testCase.authorization {
			t.Errorf("XDG_CONFIG_HOME=%s: registry.corp authorization %q, want %q", testCase.xdgConfigHome, got, testCase.authorization)
		}
	}
}
