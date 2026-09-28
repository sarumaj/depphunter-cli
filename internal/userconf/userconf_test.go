package userconf

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// machine is a machine with home as its home directory, vars as its whole
// environment (listed as well as answered) and goos as its platform. The system-wide
// paths are looked for under a directory of the test's own.
func machine(t *testing.T, home, goos string, vars map[string]string) Machine {
	t.Helper()
	saved := SystemRoot
	SystemRoot = t.TempDir()
	t.Cleanup(func() { SystemRoot = saved })
	return Machine{Home: home, GOOS: goos, Env: func(k string) string { return vars[k] }, Environ: func() []string {
		var out []string
		for k, v := range vars {
			out = append(out, k+"="+v)
		}
		sort.Strings(out)
		return out
	}}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// The user config directory is the platform's, as os.UserConfigDir finds it, from the
// machine's own home and environment.
//
// Verifies: REQ-SUP-064
func TestConfigDir(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	for _, tc := range []struct {
		goos string
		vars map[string]string
		want string
	}{
		{"linux", nil, filepath.Join(home, ".config")},
		{"linux", map[string]string{"XDG_CONFIG_HOME": xdg}, xdg},
		{"linux", map[string]string{"XDG_CONFIG_HOME": "relative"}, filepath.Join(home, ".config")},
		{"darwin", map[string]string{"XDG_CONFIG_HOME": "/xdg"}, filepath.Join(home, "Library", "Application Support")},
		{"windows", map[string]string{"APPDATA": `C:\Users\u\AppData\Roaming`}, `C:\Users\u\AppData\Roaming`},
	} {
		if got := machine(t, home, tc.goos, tc.vars).ConfigDir(); got != tc.want {
			t.Errorf("%s %v: %q, want %q", tc.goos, tc.vars, got, tc.want)
		}
	}
}

// CARGO_HOME moves Cargo's home; of config and config.toml Cargo uses the one without
// an extension when both exist; CARGO_REGISTRIES_<NAME>_INDEX names registries.
//
// Verifies: REQ-SUP-064
func TestCargoLocations(t *testing.T) {
	home, cargo := t.TempDir(), t.TempDir()
	m := machine(t, home, "linux", nil)
	if got := m.CargoFile("config"); got != filepath.Join(home, ".cargo", "config.toml") {
		t.Errorf("default: %s", got)
	}
	m = machine(t, home, "linux", map[string]string{
		"CARGO_HOME":                      cargo,
		"CARGO_REGISTRIES_MY_REG_INDEX":   "sparse+https://crates.corp/index/",
		"CARGO_REGISTRIES_EMPTY_INDEX":    "",
		"CARGO_REGISTRIES_OTHER_TOKEN":    "t",
		"CARGO_REGISTRIES_OTHER_PROTOCOL": "sparse",
	})
	if got := m.CargoFile("credentials"); got != filepath.Join(cargo, "credentials.toml") {
		t.Errorf("CARGO_HOME: %s", got)
	}
	touch(t, filepath.Join(cargo, "credentials"))
	touch(t, filepath.Join(cargo, "credentials.toml"))
	if got := m.CargoFile("credentials"); got != filepath.Join(cargo, "credentials") {
		t.Errorf("both exist: %s, want the one without an extension", got)
	}
	want := map[string]string{"MY_REG": "sparse+https://crates.corp/index/"}
	if got := m.CargoRegistries(); !reflect.DeepEqual(got, want) {
		t.Errorf("registries: %v, want %v", got, want)
	}
	if CargoRegistryName("my-reg") != "MY_REG" {
		t.Error("my-reg is not MY_REG")
	}
}

// npm reads npm_config_* in any case, the lower-case spelling winning, with "_" as
// "-"; a //host/:field key is kept as written. userconfig and globalconfig move the
// two npmrc files.
//
// Verifies: REQ-SUP-064
func TestNpmEnvironment(t *testing.T) {
	home := t.TempDir()
	m := machine(t, home, "linux", map[string]string{
		"NPM_CONFIG_REGISTRY":               "https://upper.corp/npm",
		"npm_config_registry":               "https://lower.corp/npm",
		"npm_config_@acme:registry":         "https://acme.corp/npm",
		"Npm_Config_Strict_SSL":             "false",
		"npm_config_//npm.corp/:_authToken": "tok",
		"NPM_CONFIG_USERCONFIG":             "/ci/npmrc",
		"npm_config_prefix":                 "/opt/node",
		"UNRELATED_npm_config_registry":     "https://nope",
	})
	got := m.NpmEnv()
	want := map[string]string{
		"registry":               "https://lower.corp/npm",
		"@acme:registry":         "https://acme.corp/npm",
		"strict-ssl":             "false",
		"//npm.corp/:_authToken": "tok",
		"userconfig":             "/ci/npmrc",
		"prefix":                 "/opt/node",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	if f := m.NpmUserConfig(); f != "/ci/npmrc" {
		t.Errorf("userconfig: %s", f)
	}
	if f := m.NpmGlobalConfig(); f != filepath.Join("/opt/node", "etc", "npmrc") {
		t.Errorf("global from prefix: %s", f)
	}

	// Without a listing of the environment the usual spellings still answer.
	m = machine(t, home, "linux", nil)
	m.Env = func(k string) string {
		return map[string]string{"NPM_CONFIG_GLOBALCONFIG": "/etc/npmrc-ci"}[k]
	}
	if f := m.NpmGlobalConfig(); f != "/etc/npmrc-ci" {
		t.Errorf("globalconfig: %s", f)
	}
	if f := m.NpmUserConfig(); f != filepath.Join(home, ".npmrc") {
		t.Errorf("default userconfig: %s", f)
	}
	if f := machine(t, home, "linux", nil).NpmGlobalConfig(); f != "" {
		t.Errorf("no prefix, no guess: %s", f)
	}
}

// pip's files in the order it loads them, per platform: site-wide, then the user's
// (legacy, then the config directory), then PIP_CONFIG_FILE - which hides the user's
// when it exists, and switches all of them off when it is the null device.
//
// Verifies: REQ-SUP-064
func TestPipConfigFiles(t *testing.T) {
	home := t.TempDir()
	m := machine(t, home, "linux", nil)
	files, ok := m.PipConfigFiles()
	want := []string{
		filepath.Join(SystemRoot, "etc", "xdg", "pip", "pip.conf"),
		filepath.Join(SystemRoot, "etc", "pip.conf"),
		filepath.Join(home, ".pip", "pip.conf"),
		filepath.Join(home, ".config", "pip", "pip.conf"),
	}
	if !ok || !reflect.DeepEqual(files, want) {
		t.Errorf("linux:\n got %v\nwant %v", files, want)
	}

	sep := string(filepath.ListSeparator)
	m = machine(t, home, "linux", map[string]string{"XDG_CONFIG_DIRS": "/a" + sep + "/b", "XDG_CONFIG_HOME": "/xdg", "PIP_CONFIG_FILE": "/missing.conf"})
	files, _ = m.PipConfigFiles()
	want = []string{
		filepath.Join("/a", "pip", "pip.conf"), filepath.Join("/b", "pip", "pip.conf"),
		filepath.Join(SystemRoot, "etc", "pip.conf"),
		filepath.Join(home, ".pip", "pip.conf"), filepath.Join("/xdg", "pip", "pip.conf"),
		"/missing.conf",
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("XDG and a missing PIP_CONFIG_FILE:\n got %v\nwant %v", files, want)
	}

	own := filepath.Join(t.TempDir(), "pip.conf")
	touch(t, own)
	m = machine(t, home, "linux", map[string]string{"PIP_CONFIG_FILE": own})
	files, _ = m.PipConfigFiles()
	want = []string{filepath.Join(SystemRoot, "etc", "xdg", "pip", "pip.conf"), filepath.Join(SystemRoot, "etc", "pip.conf"), own}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("existing PIP_CONFIG_FILE:\n got %v\nwant %v", files, want)
	}

	if _, ok := machine(t, home, "linux", map[string]string{"PIP_CONFIG_FILE": "/dev/null"}).PipConfigFiles(); ok {
		t.Error("PIP_CONFIG_FILE=/dev/null did not switch the files off")
	}
	if _, ok := machine(t, home, "windows", map[string]string{"PIP_CONFIG_FILE": "NUL"}).PipConfigFiles(); ok {
		t.Error("PIP_CONFIG_FILE=NUL did not switch the files off on Windows")
	}

	m = machine(t, home, "darwin", nil)
	files, _ = m.PipConfigFiles()
	want = []string{
		filepath.Join(SystemRoot, "Library", "Application Support", "pip", "pip.conf"),
		filepath.Join(home, ".pip", "pip.conf"),
		filepath.Join(home, ".config", "pip", "pip.conf"),
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("macOS without Application Support/pip:\n got %v\nwant %v", files, want)
	}
	support := filepath.Join(home, "Library", "Application Support", "pip")
	if err := os.MkdirAll(support, 0o755); err != nil {
		t.Fatal(err)
	}
	if files, _ = m.PipConfigFiles(); files[len(files)-1] != filepath.Join(support, "pip.conf") {
		t.Errorf("macOS with Application Support/pip: %v", files)
	}

	m = machine(t, home, "windows", map[string]string{"APPDATA": `C:\AppData`, "ProgramData": `C:\ProgramData`})
	files, _ = m.PipConfigFiles()
	want = []string{
		filepath.Join(`C:\ProgramData`, "pip", "pip.ini"),
		filepath.Join(home, "pip", "pip.ini"),
		filepath.Join(`C:\AppData`, "pip", "pip.ini"),
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("windows:\n got %v\nwant %v", files, want)
	}
}

// Registry credential files in the order containers-auth.json(5) consults them.
//
// Verifies: REQ-AUTH-020
func TestContainerAuthFiles(t *testing.T) {
	home := t.TempDir()
	for _, tc := range []struct {
		goos string
		vars map[string]string
		want []string
	}{
		{"linux", map[string]string{"REGISTRY_AUTH_FILE": "/ci/auth.json", "DOCKER_CONFIG": "/d"}, []string{"/ci/auth.json"}},
		{"linux", map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000", "DOCKER_CONFIG": "/d"}, []string{
			filepath.Join("/run/user/1000", "containers", "auth.json"),
			filepath.Join(home, ".config", "containers", "auth.json"),
			filepath.Join("/d", "config.json"),
		}},
		{"linux", map[string]string{"XDG_CONFIG_HOME": "/xdg"}, []string{
			filepath.Join("/xdg", "containers", "auth.json"),
			filepath.Join(home, ".docker", "config.json"),
		}},
		{"darwin", nil, []string{
			filepath.Join(home, ".config", "containers", "auth.json"),
			filepath.Join(home, ".docker", "config.json"),
		}},
	} {
		if got := machine(t, home, tc.goos, tc.vars).ContainerAuthFiles(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s %v:\n got %v\nwant %v", tc.goos, tc.vars, got, tc.want)
		}
	}
}

// NETRC names the file; else Windows prefers _netrc when it exists.
//
// Verifies: REQ-AUTH-002
func TestNetrcLocation(t *testing.T) {
	home := t.TempDir()
	if got := machine(t, home, "linux", map[string]string{"NETRC": "/ci/netrc"}).Netrc(); got != "/ci/netrc" {
		t.Errorf("NETRC: %s", got)
	}
	if got := machine(t, home, "windows", nil).Netrc(); got != filepath.Join(home, ".netrc") {
		t.Errorf("windows without _netrc: %s", got)
	}
	touch(t, filepath.Join(home, "_netrc"))
	if got := machine(t, home, "windows", nil).Netrc(); got != filepath.Join(home, "_netrc") {
		t.Errorf("windows with _netrc: %s", got)
	}
	if got := machine(t, home, "linux", nil).Netrc(); got != filepath.Join(home, ".netrc") {
		t.Errorf("linux: %s", got)
	}
}

// A Go setting comes from the environment when set there, else from the go env file
// (GOENV, else go/env in the user config directory); GOENV=off has no file.
//
// Verifies: REQ-SUP-064, REQ-SUP-036
func TestGoEnv(t *testing.T) {
	home := t.TempDir()
	file := filepath.Join(home, ".config", "go", "env")
	touch(t, file)
	if err := os.WriteFile(file, []byte("GOPRIVATE=corp.example/*\r\n# note\nGOPROXY=https://first\nGOPROXY=https://goproxy.corp,direct\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := machine(t, home, "linux", nil)
	if got := m.GoEnv("GOPROXY"); got != "https://goproxy.corp,direct" {
		t.Errorf("GOPROXY from the file: %q", got)
	}
	if got := m.GoEnv("GOPRIVATE"); got != "corp.example/*" {
		t.Errorf("GOPRIVATE from the file: %q", got)
	}
	if got := m.GoEnv("GONOSUMDB"); got != "" {
		t.Errorf("GONOSUMDB: %q", got)
	}
	m = machine(t, home, "linux", map[string]string{"GOPROXY": "https://env.proxy"})
	if got := m.GoEnv("GOPROXY"); got != "https://env.proxy" {
		t.Errorf("environment over the file: %q", got)
	}
	if got := machine(t, home, "linux", map[string]string{"GOENV": "off"}).GoEnv("GOPRIVATE"); got != "" {
		t.Errorf("GOENV=off: %q", got)
	}
	other := filepath.Join(t.TempDir(), "goenv")
	if err := os.WriteFile(other, []byte("GOPRIVATE=other.example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := machine(t, home, "linux", map[string]string{"GOENV": other}).GoEnv("GOPRIVATE"); got != "other.example" {
		t.Errorf("GOENV: %q", got)
	}
	if got := machine(t, home, "windows", map[string]string{"APPDATA": `C:\AppData`}).GoEnvFile(); got != filepath.Join(`C:\AppData`, "go", "env") {
		t.Errorf("windows: %s", got)
	}
}

// NuGet's user configuration is under %APPDATA% on Windows.
//
// Verifies: REQ-SUP-064
func TestNuGetConfigs(t *testing.T) {
	home := t.TempDir()
	if got := machine(t, home, "windows", map[string]string{"APPDATA": `C:\AppData`}).NuGetConfigs(); !reflect.DeepEqual(got, []string{filepath.Join(`C:\AppData`, "NuGet", "NuGet.Config")}) {
		t.Errorf("windows: %v", got)
	}
	if got := machine(t, home, "linux", nil).NuGetConfigs(); len(got) != 2 {
		t.Errorf("linux: %v", got)
	}
}

// Composer takes one home: COMPOSER_HOME, %APPDATA%\Composer on Windows, else the
// first existing of the XDG one and ~/.composer.
//
// Verifies: REQ-SUP-064, REQ-AUTH-016
func TestComposerHome(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	if got := machine(t, home, "linux", nil).ComposerHome(); got != "" {
		t.Errorf("none exists: %q", got)
	}
	if err := os.MkdirAll(filepath.Join(home, ".composer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := machine(t, home, "linux", nil).ComposerHome(); got != filepath.Join(home, ".composer") {
		t.Errorf("legacy: %q", got)
	}
	if err := os.MkdirAll(filepath.Join(xdg, "composer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := machine(t, home, "linux", map[string]string{"XDG_CONFIG_HOME": xdg}).ComposerHome(); got != filepath.Join(xdg, "composer") {
		t.Errorf("XDG: %q", got)
	}
	if got := machine(t, home, "windows", map[string]string{"APPDATA": `C:\AppData`}).ComposerHome(); got != filepath.Join(`C:\AppData`, "Composer") {
		t.Errorf("windows: %q", got)
	}
	if got := machine(t, home, "windows", map[string]string{"COMPOSER_HOME": "/c"}).ComposerHome(); got != "/c" {
		t.Errorf("COMPOSER_HOME: %q", got)
	}
}

// Without a home directory nothing under it is named.
func TestNoHome(t *testing.T) {
	m := machine(t, "", "linux", nil)
	for name, got := range map[string]string{
		"cargo": m.CargoFile("config"), "npmrc": m.NpmUserConfig(), "netrc": m.Netrc(),
		"bundler": m.BundlerConfig(), "composer": m.ComposerHome(),
	} {
		if got != "" {
			t.Errorf("%s: %q", name, got)
		}
	}
	if New("", nil).Env("HOME") != "" {
		t.Error("a nil environment answered")
	}
}

// New gives the machine the platform Platform names - runtime.GOOS unless a test
// pinned it - so tests whose fixtures sit where one platform keeps the files pass
// on every platform.
//
// Verifies: REQ-SUP-064
func TestNewPlatform(t *testing.T) {
	saved := Platform
	t.Cleanup(func() { Platform = saved })
	Platform = "darwin"
	if got := New("h", nil).ConfigDir(); got != filepath.Join("h", "Library", "Application Support") {
		t.Errorf("pinned darwin: ConfigDir = %q", got)
	}
}
