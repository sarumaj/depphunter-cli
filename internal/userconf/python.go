package userconf

import (
	"cmp"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------- uv

// UVNoConfig reports whether UV_NO_CONFIG switches uv's configuration files off: uv
// then reads no uv.toml, the user's, the system's or a project's.
//
// Implements: REQ-SUP-064
func (m Machine) UVNoConfig() bool {
	switch strings.ToLower(strings.TrimSpace(m.Environment("UV_NO_CONFIG"))) {
	case "1", "true", "yes", "on", "y", "t":
		return true
	}
	return false
}

// UVConfigFiles are the uv.toml files uv reads for this user, the one whose settings
// win first: the file UV_CONFIG_FILE names alone, when it is set; else the user's
// ($XDG_CONFIG_HOME/uv/uv.toml or ~/.config/uv/uv.toml; %APPDATA%\uv\uv.toml on
// Windows) and the system's (the first $XDG_CONFIG_DIRS/uv/uv.toml that exists,
// else /etc/uv/uv.toml; %ProgramData%\uv\uv.toml on Windows). None under
// UV_NO_CONFIG.
//
// Implements: REQ-SUP-064
func (m Machine) UVConfigFiles() []string {
	if m.UVNoConfig() {
		return nil
	}
	if f := m.Environment("UV_CONFIG_FILE"); f != "" {
		return []string{f}
	}
	files := []string{cmp.Or(m.on(m.under("APPDATA", "uv", "uv.toml"), "windows"), m.xdgConfigHome("uv", "uv.toml"))}
	if m.GOOS == "windows" {
		return append(files, present(m.under("ProgramData", "uv", "uv.toml"))...)
	}
	var systemFiles []string
	for _, d := range filepath.SplitList(m.Environment("XDG_CONFIG_DIRS")) {
		systemFiles = append(systemFiles, join(d, "uv", "uv.toml"))
	}
	return append(files, cmp.Or(firstFile(systemFiles...), system("etc", "uv", "uv.toml")))
}

// UVProjectConfig reports whether a project's uv.toml is read: not under
// UV_NO_CONFIG, and not when UV_CONFIG_FILE names the one file uv reads.
//
// Implements: REQ-SUP-064
func (m Machine) UVProjectConfig() bool {
	return !m.UVNoConfig() && m.Environment("UV_CONFIG_FILE") == ""
}

// ---------------------------------------------------------------- Poetry

// PoetryConfigDirectory is the directory of Poetry's config.toml and auth.toml:
// POETRY_CONFIG_DIR; %APPDATA%\pypoetry on Windows; ~/Library/Application
// Support/pypoetry on macOS; else $XDG_CONFIG_HOME/pypoetry (~/.config/pypoetry).
//
// Implements: REQ-SUP-064
func (m Machine) PoetryConfigDirectory() string {
	return cmp.Or(m.Environment("POETRY_CONFIG_DIR"),
		m.byPlatform(m.under("APPDATA", "pypoetry"), m.applicationSupport("pypoetry"), m.xdgConfigHome("pypoetry")))
}

// PoetryRepositoryVariables are the repositories the POETRY_REPOSITORIES_<NAME>_URL
// variables define, by <NAME>.
//
// Implements: REQ-SUP-064
func (m Machine) PoetryRepositoryVariables() map[string]string {
	const prefix, suffix = "POETRY_REPOSITORIES_", "_URL"
	out := map[string]string{}
	for _, name := range m.names(func(n string) bool {
		return len(n) > len(prefix)+len(suffix) && strings.HasPrefix(n, prefix) && strings.HasSuffix(n, suffix)
	}) {
		if v := m.Environment(name); v != "" {
			out[name[len(prefix):len(name)-len(suffix)]] = v
		}
	}
	return out
}

// ---------------------------------------------------------------- PDM

// PDMConfigFile is PDM's global config.toml: PDM_CONFIG_FILE; %LOCALAPPDATA%\pdm\pdm
// on Windows; ~/Library/Application Support/pdm on macOS; else
// $XDG_CONFIG_HOME/pdm (~/.config/pdm).
//
// Implements: REQ-SUP-064
func (m Machine) PDMConfigFile() string {
	return cmp.Or(m.Environment("PDM_CONFIG_FILE"), m.byPlatform(m.under("LOCALAPPDATA", "pdm", "pdm", "config.toml"),
		m.applicationSupport("pdm", "config.toml"), m.xdgConfigHome("pdm", "config.toml")))
}
