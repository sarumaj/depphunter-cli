package userconf

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// ---------------------------------------------------------------- dub

// DubUserDir is where dub keeps the user's settings.json: `DUB_HOME`; else dub
// below `DPATH`; else %APPDATA%\dub on Windows (when %APPDATA% is set); else
// ~/.dub. A `dubHome` in the system settings, which moves it too, is not followed.
//
// Implements: REQ-SUP-064
func (m Machine) DubUserDir() string {
	if dir := m.Env("DUB_HOME"); dir != "" {
		return dir
	}
	if dir := m.Env("DPATH"); dir != "" {
		return filepath.Join(dir, "dub")
	}
	if m.GOOS == "windows" {
		if dir := m.Env("APPDATA"); dir != "" {
			return filepath.Join(dir, "dub")
		}
	}
	return join(m.Home, ".dub")
}

// DubSettings lists dub's settings.json files, the one dub gives priority first: the
// user's (DubUserDir), then the system's - %ProgramData%\dub on Windows, else
// /etc/dub (where a dub installed under /usr looks) and /var/lib/dub. The file
// beside the dub executable (../etc/dub) is not looked for.
//
// Implements: REQ-SUP-064
func (m Machine) DubSettings() []string {
	var out []string
	if dir := m.DubUserDir(); dir != "" {
		out = append(out, filepath.Join(dir, "settings.json"))
	}
	if m.GOOS == "windows" {
		if dir := m.Env("ProgramData"); dir != "" {
			out = append(out, filepath.Join(dir, "dub", "settings.json"))
		}
		return out
	}
	return append(out, system("etc", "dub", "settings.json"), system("var", "lib", "dub", "settings.json"))
}

// ---------------------------------------------------------------- Quicklisp

// QuicklispDists lists the distinfo.txt of every dist installed in the user's
// Quicklisp (~/quicklisp/dists/<name>/distinfo.txt), by name.
//
// Implements: REQ-SUP-064
func (m Machine) QuicklispDists() []string {
	dir := join(m.Home, "quicklisp", "dists")
	if dir == "" {
		return nil
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*", "distinfo.txt"))
	sort.Strings(files)
	return files
}

// ---------------------------------------------------------------- opam

// OpamRoot is opam's root directory, which holds its configuration and a copy
// of every repository it has fetched: `OPAMROOT`; else %LOCALAPPDATA%\opam on
// Windows (opam 2.2's default) when %LOCALAPPDATA% is set; else ~/.opam.
//
// Implements: REQ-SUP-064
func (m Machine) OpamRoot() string {
	if dir := m.Env("OPAMROOT"); dir != "" {
		return dir
	}
	if m.GOOS == "windows" {
		if dir := m.Env("LOCALAPPDATA"); dir != "" {
			return filepath.Join(dir, "opam")
		}
	}
	return join(m.Home, ".opam")
}

// ---------------------------------------------------------------- Alire

// AlireSettingsDir is where Alire keeps its settings and its indexes:
// `ALIRE_SETTINGS_DIR` (Alire 2), else `ALR_CONFIG` (Alire 1); else
// %USERPROFILE%\.config\alire on Windows, else $XDG_CONFIG_HOME/alire
// (~/.config/alire).
//
// Implements: REQ-SUP-064
func (m Machine) AlireSettingsDir() string {
	for _, v := range []string{"ALIRE_SETTINGS_DIR", "ALR_CONFIG"} {
		if dir := m.Env(v); dir != "" {
			return dir
		}
	}
	if m.GOOS == "windows" {
		return join(m.Home, ".config", "alire")
	}
	return join(m.xdgConfigHome(), "alire")
}

// ---------------------------------------------------------------- Julia

// JuliaDepots lists the Julia depots whose registries Pkg reads, in order:
// the entries of `JULIA_DEPOT_PATH` (`;`-separated on Windows, else
// `:`-separated; `~` is the home directory), where an empty entry stands for
// the default depot, else the default depot ~/.julia alone. The depots bundled
// with a Julia installation, which an empty entry also brings in, are not
// known here: where Julia is installed is not. A depot is listed once.
//
// Implements: REQ-SUP-055, REQ-SUP-064
func (m Machine) JuliaDepots() []string {
	user := join(m.Home, ".julia")
	sep := ":"
	if m.GOOS == "windows" {
		sep = ";"
	}
	var out []string
	add := func(d string) {
		if d != "" && !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	value := m.Env("JULIA_DEPOT_PATH")
	if value == "" {
		add(user)
		return out
	}
	for _, d := range strings.Split(value, sep) {
		switch {
		case d == "":
			d = user
		case d == "~":
			d = m.Home
		case strings.HasPrefix(d, "~/") || strings.HasPrefix(d, `~\`):
			d = join(m.Home, d[2:])
		}
		add(d)
	}
	return out
}
