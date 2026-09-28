package userconf

import (
	"path/filepath"
	"sort"
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
