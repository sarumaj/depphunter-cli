package userconf

import (
	"cmp"
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
	return cmp.Or(m.Environment("DUB_HOME"), m.under("DPATH", "dub"),
		m.on(m.under("APPDATA", "dub"), "windows"), m.home(".dub"))
}

// DubSettings lists dub's settings.json files, the one dub gives priority first: the
// user's (DubUserDirectory), then the system's - %ProgramData%\dub on Windows, else
// /etc/dub (where a dub installed under /usr looks) and /var/lib/dub. The file
// beside the dub executable (../etc/dub) is not looked for.
//
// Implements: REQ-SUP-064
func (m Machine) DubSettings() []string {
	out := present(join(m.DubUserDirectory(), "settings.json"))
	if m.GOOS == "windows" {
		return append(out, present(m.under("ProgramData", "dub", "settings.json"))...)
	}
	return append(out, system("etc", "dub", "settings.json"), system("var", "lib", "dub", "settings.json"))
}

// ---------------------------------------------------------------- Quicklisp

// QuicklispDists lists the distinfo.txt of every dist installed in the user's
// Quicklisp (~/quicklisp/dists/<name>/distinfo.txt), by name.
//
// Implements: REQ-SUP-064
func (m Machine) QuicklispDists() []string {
	directory := m.home("quicklisp", "dists")
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
	return cmp.Or(m.Environment("OPAMROOT"), m.on(m.under("LOCALAPPDATA", "opam"), "windows"), m.home(".opam"))
}

// ---------------------------------------------------------------- Alire

// AlireSettingsDirectory is where Alire keeps its settings and its indexes:
// `ALIRE_SETTINGS_DIR` (Alire 2), else `ALR_CONFIG` (Alire 1); else
// %USERPROFILE%\.config\alire on Windows, else $XDG_CONFIG_HOME/alire
// (~/.config/alire).
//
// Implements: REQ-SUP-064
func (m Machine) AlireSettingsDirectory() string {
	directory := m.xdgConfigHome("alire")
	if m.GOOS == "windows" {
		directory = m.home(".config", "alire")
	}
	return cmp.Or(m.Environment("ALIRE_SETTINGS_DIR"), m.Environment("ALR_CONFIG"), directory)
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
	user := m.home(".julia")
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
			d = m.home(d[2:])
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
	return cmp.Or(m.Environment("CUE_CONFIG_DIR"), join(m.ConfigDirectory(), "cue"))
}

// ---------------------------------------------------------------- CocoaPods and SwiftPM

// CocoaPodsRepositories is the directory holding CocoaPods' spec repositories,
// one directory each (`pod repo add` clones a git one there): `CP_REPOS_DIR`,
// else repos below `CP_HOME_DIR`, else ~/.cocoapods/repos.
//
// Implements: REQ-SUP-064
func (m Machine) CocoaPodsRepositories() string {
	return cmp.Or(m.Environment("CP_REPOS_DIR"), m.under("CP_HOME_DIR", "repos"), m.home(".cocoapods", "repos"))
}

// SwiftPMRegistries is the user's registries.json, which `swift package-registry
// set --global` writes: in the configuration directory SwiftPM calls
// idiomatic, ~/Library/org.swift.swiftpm/configuration on macOS; else (and on
// macOS when that has none, as before SwiftPM 5.6) configuration below
// $XDG_CONFIG_HOME/swiftpm when XDG_CONFIG_HOME is set, else below ~/.swiftpm.
//
// Implements: REQ-SUP-064
func (m Machine) SwiftPMRegistries() string {
	idiomatic := m.on(firstFile(m.home("Library", "org.swift.swiftpm", "configuration", "registries.json")), "darwin")
	directory := cmp.Or(m.under("XDG_CONFIG_HOME", "swiftpm"), m.home(".swiftpm"))
	return cmp.Or(idiomatic, join(directory, "configuration", "registries.json"))
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
		return m.home(".conan2")
	case directory == "~":
		return m.Home
	case strings.HasPrefix(directory, "~/") || strings.HasPrefix(directory, `~\`):
		return m.home(directory[2:])
	case filepath.IsAbs(directory), strings.HasPrefix(directory, "/"), m.GOOS == "windows" && windowsAbsolute(directory):
		return directory
	}
	return ""
}

// ---------------------------------------------------------------- cabal and LuaRocks

// CabalConfig is the configuration file cabal-install reads: `CABAL_CONFIG`, else
// config in `CABAL_DIR`, else ~/.cabal/config when ~/.cabal exists and the XDG
// file does not (an installation from before cabal 3.10), else config in the
// cabal directory of $XDG_CONFIG_HOME or ~/.config. On Windows both are
// %APPDATA%\cabal\config.
//
// Implements: REQ-SUP-064
func (m Machine) CabalConfig() string {
	if name := cmp.Or(m.Environment("CABAL_CONFIG"), m.under("CABAL_DIR", "config"),
		m.on(m.under("APPDATA", "cabal", "config"), "windows")); name != "" {
		return name
	}
	xdg := m.xdgConfigHome("cabal", "config")
	if legacy := m.home(".cabal"); legacy != "" && isDirectory(legacy) && !isFile(xdg) {
		return filepath.Join(legacy, "config")
	}
	return xdg
}

// luaVersions are the Lua versions whose LuaRocks configuration is read: which one
// a project's rocks are installed for is not known here.
var luaVersions = []string{"5.1", "5.2", "5.3", "5.4"}

// LuaRocksConfigs are the user configuration files LuaRocks reads, one per Lua
// version: the file `LUAROCKS_CONFIG_5_x`, else `LUAROCKS_CONFIG`, names when it
// exists; else config-5.x.lua in $XDG_CONFIG_HOME/luarocks (~/.config) when that
// exists, else in ~/.luarocks; on Windows in %APPDATA%\luarocks. Each file
// replaces the rocks_servers of the system's.
//
// Implements: REQ-SUP-064
func (m Machine) LuaRocksConfigs() []string {
	var out []string
	for _, version := range luaVersions {
		name := "config-" + version + ".lua"
		user := []string{m.xdgConfigHome("luarocks", name), m.home(".luarocks", name)}
		if appData := m.Environment("APPDATA"); m.GOOS == "windows" && appData != "" {
			user = []string{filepath.Join(appData, "luarocks", name)}
		}
		versioned := m.Environment("LUAROCKS_CONFIG_" + strings.ReplaceAll(version, ".", "_"))
		candidates := append([]string{versioned, m.Environment("LUAROCKS_CONFIG")}, user...)
		if file := firstFile(candidates...); file != "" && !slices.Contains(out, file) {
			out = append(out, file)
		}
	}
	return out
}
