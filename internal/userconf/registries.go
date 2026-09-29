package userconf

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// ---------------------------------------------------------------- dub

// DubUserDirectory is where dub keeps the user's settings.json: `DUB_HOME`; else dub
// below `DPATH`; else %APPDATA%\dub on Windows (when %APPDATA% is set); else
// ~/.dub. A `dubHome` in the system settings, which moves it too, is not followed.
//
// Implements: REQ-SUP-064
func (m Machine) DubUserDirectory() string {
	if directory := m.Environment("DUB_HOME"); directory != "" {
		return directory
	}
	if directory := m.Environment("DPATH"); directory != "" {
		return filepath.Join(directory, "dub")
	}
	if m.GOOS == "windows" {
		if directory := m.Environment("APPDATA"); directory != "" {
			return filepath.Join(directory, "dub")
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
	if directory := m.DubUserDirectory(); directory != "" {
		out = append(out, filepath.Join(directory, "settings.json"))
	}
	if m.GOOS == "windows" {
		if directory := m.Environment("ProgramData"); directory != "" {
			out = append(out, filepath.Join(directory, "dub", "settings.json"))
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
	directory := join(m.Home, "quicklisp", "dists")
	if directory == "" {
		return nil
	}
	files, _ := filepath.Glob(filepath.Join(directory, "*", "distinfo.txt"))
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
	if directory := m.Environment("OPAMROOT"); directory != "" {
		return directory
	}
	if m.GOOS == "windows" {
		if directory := m.Environment("LOCALAPPDATA"); directory != "" {
			return filepath.Join(directory, "opam")
		}
	}
	return join(m.Home, ".opam")
}

// ---------------------------------------------------------------- Alire

// AlireSettingsDirectory is where Alire keeps its settings and its indexes:
// `ALIRE_SETTINGS_DIR` (Alire 2), else `ALR_CONFIG` (Alire 1); else
// %USERPROFILE%\.config\alire on Windows, else $XDG_CONFIG_HOME/alire
// (~/.config/alire).
//
// Implements: REQ-SUP-064
func (m Machine) AlireSettingsDirectory() string {
	for _, v := range []string{"ALIRE_SETTINGS_DIR", "ALR_CONFIG"} {
		if directory := m.Environment(v); directory != "" {
			return directory
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
	separator := ":"
	if m.GOOS == "windows" {
		separator = ";"
	}
	var out []string
	add := func(d string) {
		if d != "" && !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	value := m.Environment("JULIA_DEPOT_PATH")
	if value == "" {
		add(user)
		return out
	}
	for _, d := range strings.Split(value, separator) {
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

// ---------------------------------------------------------------- Puppet and CUE

// R10KConfigs lists the r10k configuration files r10k reads when it is not given
// one, in its order: /etc/puppetlabs/r10k/r10k.yaml, then the older /etc/r10k.yaml
// (the first that exists is the one r10k uses).
//
// Implements: REQ-SUP-064
func (m Machine) R10KConfigs() []string {
	return []string{system("etc", "puppetlabs", "r10k", "r10k.yaml"), system("etc", "r10k.yaml")}
}

// CUEConfigDirectory is where cue keeps its configuration (logins.json):
// `CUE_CONFIG_DIR`, else cue in the platform's configuration directory
// (os.UserConfigDir).
//
// Implements: REQ-SUP-064
func (m Machine) CUEConfigDirectory() string {
	if directory := m.Environment("CUE_CONFIG_DIR"); directory != "" {
		return directory
	}
	return join(m.ConfigDirectory(), "cue")
}

// ---------------------------------------------------------------- CocoaPods and SwiftPM

// CocoaPodsRepositories is the directory holding CocoaPods' spec repositories,
// one directory each (`pod repo add` clones a git one there): `CP_REPOS_DIR`,
// else repos below `CP_HOME_DIR`, else ~/.cocoapods/repos.
//
// Implements: REQ-SUP-064
func (m Machine) CocoaPodsRepositories() string {
	if directory := m.Environment("CP_REPOS_DIR"); directory != "" {
		return directory
	}
	if directory := m.Environment("CP_HOME_DIR"); directory != "" {
		return filepath.Join(directory, "repos")
	}
	return join(m.Home, ".cocoapods", "repos")
}

// SwiftPMRegistries is the user's registries.json, which `swift package-registry
// set --global` writes: in the configuration directory SwiftPM calls
// idiomatic, ~/Library/org.swift.swiftpm/configuration on macOS; else (and on
// macOS when that has none, as before SwiftPM 5.6) configuration below
// $XDG_CONFIG_HOME/swiftpm when XDG_CONFIG_HOME is set, else below ~/.swiftpm.
//
// Implements: REQ-SUP-064
func (m Machine) SwiftPMRegistries() string {
	if m.GOOS == "darwin" {
		if idiomatic := join(m.Home, "Library", "org.swift.swiftpm", "configuration", "registries.json"); idiomatic != "" && isFile(idiomatic) {
			return idiomatic
		}
	}
	directory := join(m.Home, ".swiftpm")
	if xdg := m.Environment("XDG_CONFIG_HOME"); xdg != "" {
		directory = filepath.Join(xdg, "swiftpm")
	}
	return join(directory, "configuration", "registries.json")
}

// ---------------------------------------------------------------- Conan

// ConanHome is Conan 2's home, which holds remotes.json and credentials.json:
// `CONAN_HOME` (a leading ~ is the user's home), else .conan2 in the user's home.
// A CONAN_HOME that is not absolute, which Conan refuses, names none. The
// .conanrc Conan looks for from the directory it runs in upwards is not read
// here: in a repository it is the repository's say (internal/index).
//
// Implements: REQ-SUP-064
func (m Machine) ConanHome() string {
	directory := strings.TrimSpace(m.Environment("CONAN_HOME"))
	switch {
	case directory == "":
		return join(m.Home, ".conan2")
	case directory == "~":
		return m.Home
	case strings.HasPrefix(directory, "~/") || strings.HasPrefix(directory, `~\`):
		return join(m.Home, directory[2:])
	case filepath.IsAbs(directory), strings.HasPrefix(directory, "/"), m.GOOS == "windows" && windowsAbsolute(directory):
		return directory
	}
	return ""
}
