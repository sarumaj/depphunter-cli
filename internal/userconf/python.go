package userconf

import (
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------- uv

// UVNoConfig reports whether UV_NO_CONFIG switches uv's configuration files off: uv
// then reads no uv.toml, the user's, the system's or a project's.
//
// Implements: REQ-SUP-064
func (m Machine) UVNoConfig() bool {
	switch strings.ToLower(strings.TrimSpace(m.Env("UV_NO_CONFIG"))) {
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
	if f := m.Env("UV_CONFIG_FILE"); f != "" {
		return []string{f}
	}
	var files []string
	if m.GOOS == "windows" && m.Env("APPDATA") != "" {
		files = append(files, join(m.Env("APPDATA"), "uv", "uv.toml"))
	} else {
		files = append(files, join(m.xdgConfigHome(), "uv", "uv.toml"))
	}
	switch {
	case m.GOOS == "windows" && m.Env("ProgramData") != "":
		files = append(files, join(m.Env("ProgramData"), "uv", "uv.toml"))
	case m.GOOS != "windows":
		system := system("etc", "uv", "uv.toml")
		for _, d := range filepath.SplitList(m.Env("XDG_CONFIG_DIRS")) {
			if f := join(d, "uv", "uv.toml"); f != "" && isFile(f) {
				system = f
				break
			}
		}
		files = append(files, system)
	}
	return files
}

// UVProjectConfig reports whether a project's uv.toml is read: not under
// UV_NO_CONFIG, and not when UV_CONFIG_FILE names the one file uv reads.
//
// Implements: REQ-SUP-064
func (m Machine) UVProjectConfig() bool {
	return !m.UVNoConfig() && m.Env("UV_CONFIG_FILE") == ""
}

// ---------------------------------------------------------------- Poetry

// PoetryConfigDir is the directory of Poetry's config.toml and auth.toml:
// POETRY_CONFIG_DIR; %APPDATA%\pypoetry on Windows; ~/Library/Application
// Support/pypoetry on macOS; else $XDG_CONFIG_HOME/pypoetry (~/.config/pypoetry).
//
// Implements: REQ-SUP-064
func (m Machine) PoetryConfigDir() string {
	if dir := m.Env("POETRY_CONFIG_DIR"); dir != "" {
		return dir
	}
	switch {
	case m.GOOS == "windows" && m.Env("APPDATA") != "":
		return join(m.Env("APPDATA"), "pypoetry")
	case m.GOOS == "darwin" || m.GOOS == "ios":
		return join(m.Home, "Library", "Application Support", "pypoetry")
	}
	return join(m.xdgConfigHome(), "pypoetry")
}

// PoetryRepositoryVars are the repositories the POETRY_REPOSITORIES_<NAME>_URL
// variables define, by <NAME>.
//
// Implements: REQ-SUP-064
func (m Machine) PoetryRepositoryVars() map[string]string {
	const prefix, suffix = "POETRY_REPOSITORIES_", "_URL"
	out := map[string]string{}
	for _, name := range m.names(func(n string) bool {
		return len(n) > len(prefix)+len(suffix) && strings.HasPrefix(n, prefix) && strings.HasSuffix(n, suffix)
	}) {
		if v := m.Env(name); v != "" {
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
	if f := m.Env("PDM_CONFIG_FILE"); f != "" {
		return f
	}
	switch {
	case m.GOOS == "windows" && m.Env("LOCALAPPDATA") != "":
		return join(m.Env("LOCALAPPDATA"), "pdm", "pdm", "config.toml")
	case m.GOOS == "darwin" || m.GOOS == "ios":
		return join(m.Home, "Library", "Application Support", "pdm", "config.toml")
	}
	return join(m.xdgConfigHome(), "pdm", "config.toml")
}
