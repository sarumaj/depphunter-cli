package userconf

import (
	"path/filepath"
	"slices"
	"testing"
)

// uv reads the user's uv.toml and the system's (the first $XDG_CONFIG_DIRS one that
// exists, else /etc/uv); UV_CONFIG_FILE names the one file read, UV_NO_CONFIG
// reads none, and neither lets a project's uv.toml count.
//
// Verifies: REQ-SUP-064
func TestUVConfigFiles(t *testing.T) {
	home, xdg, directories := t.TempDir(), t.TempDir(), t.TempDir()
	m := machine(t, home, "linux", nil)
	want := []string{filepath.Join(home, ".config", "uv", "uv.toml"), filepath.Join(SystemRoot, "etc", "uv", "uv.toml")}
	if got := m.UVConfigFiles(); !slices.Equal(got, want) {
		t.Errorf("default: %v", got)
	}
	touch(t, filepath.Join(directories, "uv", "uv.toml"))
	m = machine(t, home, "linux", map[string]string{"XDG_CONFIG_HOME": xdg,
		"XDG_CONFIG_DIRS": filepath.Join(directories, "none") + string(filepath.ListSeparator) + directories})
	want = []string{filepath.Join(xdg, "uv", "uv.toml"), filepath.Join(directories, "uv", "uv.toml")}
	if got := m.UVConfigFiles(); !slices.Equal(got, want) {
		t.Errorf("XDG: %v", got)
	}
	if !m.UVProjectConfig() {
		t.Error("a project's uv.toml is read by default")
	}
	m = machine(t, home, "linux", map[string]string{"UV_CONFIG_FILE": "/ci/uv.toml"})
	if got := m.UVConfigFiles(); !slices.Equal(got, []string{"/ci/uv.toml"}) || m.UVProjectConfig() {
		t.Errorf("UV_CONFIG_FILE: %v", got)
	}
	m = machine(t, home, "linux", map[string]string{"UV_NO_CONFIG": "1"})
	if got := m.UVConfigFiles(); got != nil || m.UVProjectConfig() {
		t.Errorf("UV_NO_CONFIG: %v", got)
	}
	appData, programData := t.TempDir(), t.TempDir()
	m = machine(t, home, "windows", map[string]string{"APPDATA": appData, "ProgramData": programData})
	want = []string{filepath.Join(appData, "uv", "uv.toml"), filepath.Join(programData, "uv", "uv.toml")}
	if got := m.UVConfigFiles(); !slices.Equal(got, want) {
		t.Errorf("windows: %v", got)
	}
	m = machine(t, home, "windows", nil)
	if got := m.UVConfigFiles(); !slices.Equal(got, []string{filepath.Join(home, ".config", "uv", "uv.toml")}) {
		t.Errorf("windows without APPDATA: %v", got)
	}
}

// Poetry's configuration directory and PDM's config.toml follow their variables,
// then the platform's configuration directory; Windows without its variables falls
// through to the XDG one.
//
// Verifies: REQ-SUP-064
func TestPoetryAndPDMLocations(t *testing.T) {
	home, xdg, local := t.TempDir(), t.TempDir(), t.TempDir()
	for _, testCase := range []struct {
		goos       string
		variables  map[string]string
		poetry, pd string
	}{
		{"linux", nil, filepath.Join(home, ".config", "pypoetry"), filepath.Join(home, ".config", "pdm", "config.toml")},
		{"linux", map[string]string{"XDG_CONFIG_HOME": xdg}, filepath.Join(xdg, "pypoetry"), filepath.Join(xdg, "pdm", "config.toml")},
		{"darwin", nil, filepath.Join(home, "Library", "Application Support", "pypoetry"),
			filepath.Join(home, "Library", "Application Support", "pdm", "config.toml")},
		{"windows", map[string]string{"APPDATA": xdg, "LOCALAPPDATA": local}, filepath.Join(xdg, "pypoetry"),
			filepath.Join(local, "pdm", "pdm", "config.toml")},
		{"windows", nil, filepath.Join(home, ".config", "pypoetry"), filepath.Join(home, ".config", "pdm", "config.toml")},
		{"linux", map[string]string{"POETRY_CONFIG_DIR": xdg, "PDM_CONFIG_FILE": "/ci/pdm.toml"}, xdg, "/ci/pdm.toml"},
	} {
		m := machine(t, home, testCase.goos, testCase.variables)
		if got := m.PoetryConfigDirectory(); got != testCase.poetry {
			t.Errorf("%s %v: poetry %s", testCase.goos, testCase.variables, got)
		}
		if got := m.PDMConfigFile(); got != testCase.pd {
			t.Errorf("%s %v: pdm %s", testCase.goos, testCase.variables, got)
		}
	}
	m := machine(t, home, "linux", map[string]string{"POETRY_REPOSITORIES_CORP_URL": "https://corp/simple", "POETRY_REPOSITORIES__URL": "x"})
	if got := m.PoetryRepositoryVariables(); len(got) != 1 || got["CORP"] != "https://corp/simple" {
		t.Errorf("POETRY_REPOSITORIES_*: %v", got)
	}
}
