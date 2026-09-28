// Package userconf finds the files the package managers of this machine keep their
// configuration in, the way each of them finds its own: a variable naming the file or
// the directory first, then the platform's place for it.
//
// Index discovery (internal/index) and the credential store (internal/auth) both go
// through here, so the feed a configuration names and the credential kept beside it
// are always read from the same file. Nothing here reads the repository.
package userconf

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// SystemRoot is where the absolute system-wide paths (/etc/pip.conf, /etc/xdg, macOS's
// /Library) are looked for. Tests point it at a directory of their own, so that
// nothing on the machine running them is read.
var SystemRoot = string(filepath.Separator)

// Platform is the GOOS New gives a machine: runtime.GOOS. Tests whose fixtures sit
// where one platform's tools keep their files pin it, so that they pass on every
// platform CI runs them on.
var Platform = runtime.GOOS

// Machine is the user whose configuration is read: the home directory, the
// environment and the platform. The environment is read through Environment only; Environ
// only lists the names, for the variables whose names cannot be known in advance, so
// an Environment that answers nothing reads nothing.
type Machine struct {
	Home        string
	Environment func(string) string
	GOOS        string
	Environ     func() []string
}

// New is the machine depphunter runs on.
func New(home string, environment func(string) string) Machine {
	if environment == nil {
		environment = func(string) string { return "" }
	}
	return Machine{Home: home, Environment: environment, GOOS: Platform, Environ: os.Environ}
}

// join is filepath.Join, or "" when base is: a path under a directory nobody named
// is no path at all.
func join(base string, element ...string) string {
	if base == "" {
		return ""
	}
	return filepath.Join(append([]string{base}, element...)...)
}

// system is an absolute system-wide path under SystemRoot.
func system(element ...string) string { return join(SystemRoot, element...) }

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// names lists the environment's variable names with the given prefix, sorted.
func (m Machine) names(match func(string) bool) []string {
	if m.Environ == nil {
		return nil
	}
	var out []string
	for _, keyValue := range m.Environ() {
		if name, _, _ := strings.Cut(keyValue, "="); name != "" && match(name) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// ConfigDirectory is os.UserConfigDir for this machine: %APPDATA% on Windows,
// ~/Library/Application Support on macOS, else $XDG_CONFIG_HOME when it is absolute
// or ~/.config.
//
// Implements: REQ-SUP-064
func (m Machine) ConfigDirectory() string {
	switch m.GOOS {
	case "windows":
		return m.Environment("APPDATA")
	case "darwin", "ios":
		return join(m.Home, "Library", "Application Support")
	}
	if xdg := m.Environment("XDG_CONFIG_HOME"); filepath.IsAbs(xdg) {
		return xdg
	}
	return join(m.Home, ".config")
}

// xdgConfigHome is $XDG_CONFIG_HOME, else ~/.config, as the tools that follow the
// XDG layout on every platform (Composer, Podman, pip on Linux) take it.
func (m Machine) xdgConfigHome() string {
	if xdg := m.Environment("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	return join(m.Home, ".config")
}

// ---------------------------------------------------------------- Cargo

// CargoHome is $CARGO_HOME, else ~/.cargo.
//
// Implements: REQ-SUP-064
func (m Machine) CargoHome() string {
	if directory := m.Environment("CARGO_HOME"); directory != "" {
		return directory
	}
	return join(m.Home, ".cargo")
}

// CargoFile is the file Cargo reads for name ("config", "credentials") in its home:
// the one without an extension when it exists - Cargo warns and uses it when both
// do - else name.toml.
//
// Implements: REQ-SUP-064
func (m Machine) CargoFile(name string) string {
	directory := m.CargoHome()
	if directory == "" {
		return ""
	}
	if legacy := filepath.Join(directory, name); isFile(legacy) {
		return legacy
	}
	return filepath.Join(directory, name+".toml")
}

// CargoRegistryName is the form a registry name takes in Cargo's variables, and in
// which two names are the same registry: upper case, "-" as "_".
func CargoRegistryName(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// CargoRegistries is what the CARGO_REGISTRIES_<NAME>_INDEX variables define: index
// URL by registry name, in the form CargoRegistryName gives. A variable overrides the
// same registry's index in config.toml, and may define one config.toml does not name.
//
// Implements: REQ-SUP-064
func (m Machine) CargoRegistries() map[string]string {
	out := map[string]string{}
	for _, name := range m.names(func(n string) bool {
		return strings.HasPrefix(n, "CARGO_REGISTRIES_") && strings.HasSuffix(n, "_INDEX")
	}) {
		registry := strings.TrimSuffix(strings.TrimPrefix(name, "CARGO_REGISTRIES_"), "_INDEX")
		if v := strings.TrimSpace(m.Environment(name)); registry != "" && v != "" {
			out[registry] = v
		}
	}
	return out
}

// ---------------------------------------------------------------- npm

// NpmEnvironment is the npm configuration set in the environment, by key. npm takes every
// variable whose name starts with npm_config_ in any case, the rest of the name lower
// case with "_" as "-" (except a leading one, and except in a //host/:field key,
// which is kept as written). Of two spellings of one key the lower-case one wins.
//
// Implements: REQ-SUP-064
func (m Machine) NpmEnvironment() map[string]string {
	const prefix = "npm_config_"
	names := m.names(func(n string) bool { return len(n) > len(prefix) && strings.EqualFold(n[:len(prefix)], prefix) })
	// The keys depphunter asks for, in the two usual spellings, so that an Environment
	// without a listing of the environment still answers them.
	for _, key := range []string{"registry", "userconfig", "globalconfig", "prefix"} {
		names = append(names, "NPM_CONFIG_"+strings.ToUpper(key), prefix+key)
	}
	// Lower-case prefix last, so that it wins.
	sort.SliceStable(names, func(i, j int) bool {
		return !strings.HasPrefix(names[i], prefix) && strings.HasPrefix(names[j], prefix)
	})
	out := map[string]string{}
	for _, name := range names {
		v := m.Environment(name)
		if v == "" {
			continue
		}
		key := name[len(prefix):]
		if !strings.HasPrefix(key, "//") {
			key = key[:1] + strings.ReplaceAll(key[1:], "_", "-")
			key = strings.ToLower(key)
		}
		out[key] = v
	}
	return out
}

// NpmUserConfig is the user's .npmrc: the userconfig setting of the environment, else
// ~/.npmrc.
//
// Implements: REQ-SUP-064
func (m Machine) NpmUserConfig() string {
	if f := m.NpmEnvironment()["userconfig"]; f != "" {
		return f
	}
	return join(m.Home, ".npmrc")
}

// NpmGlobalConfig is npm's global npmrc: the globalconfig setting of the environment,
// else etc/npmrc under the prefix the environment sets. npm's own default prefix is
// where node is installed, which is not guessed here: "" then.
//
// Implements: REQ-SUP-064
func (m Machine) NpmGlobalConfig() string {
	environment := m.NpmEnvironment()
	if f := environment["globalconfig"]; f != "" {
		return f
	}
	return join(environment["prefix"], "etc", "npmrc")
}

// YarnRCFilename is the name Yarn Berry gives its configuration files, in the
// home directory and in a project alike: YARN_RC_FILENAME, else .yarnrc.yml.
//
// Implements: REQ-SUP-064
func (m Machine) YarnRCFilename() string {
	if name := m.Environment("YARN_RC_FILENAME"); name != "" && !strings.ContainsAny(name, `/\`) {
		return name
	}
	return ".yarnrc.yml"
}

// YarnUserConfig is Yarn Berry's configuration in the home directory.
//
// Implements: REQ-SUP-064
func (m Machine) YarnUserConfig() string { return join(m.Home, m.YarnRCFilename()) }

// YarnClassicConfig is Yarn 1's ~/.yarnrc.
//
// Implements: REQ-SUP-064
func (m Machine) YarnClassicConfig() string { return join(m.Home, ".yarnrc") }

// BunConfig is Bun's global bunfig: $XDG_CONFIG_HOME/.bunfig.toml when that
// exists, else ~/.bunfig.toml.
//
// Implements: REQ-SUP-064
func (m Machine) BunConfig() string {
	if xdg := m.Environment("XDG_CONFIG_HOME"); xdg != "" && isFile(filepath.Join(xdg, ".bunfig.toml")) {
		return filepath.Join(xdg, ".bunfig.toml")
	}
	return join(m.Home, ".bunfig.toml")
}

// ---------------------------------------------------------------- pip

// PipConfigFiles are the configuration files pip loads, in the order it loads them,
// each overriding the one before: the site-wide ones, the user's (the legacy
// ~/.pip/pip.conf, then the one in the user config directory) unless PIP_CONFIG_FILE
// names a file that exists, and PIP_CONFIG_FILE. The environment's PIP_* variables
// override them all. PIP_CONFIG_FILE set to the null device switches every file off:
// ok is false then. The file of the running interpreter's prefix is not known here.
//
// Implements: REQ-SUP-064
func (m Machine) PipConfigFiles() (files []string, ok bool) {
	environment := m.Environment("PIP_CONFIG_FILE")
	if environment == "/dev/null" && m.GOOS != "windows" || m.GOOS == "windows" && strings.EqualFold(environment, "nul") {
		return nil, false
	}
	base := "pip.conf"
	if m.GOOS == "windows" {
		base = "pip.ini"
	}
	switch m.GOOS {
	case "windows":
		files = append(files, join(m.Environment("ProgramData"), "pip", base))
	case "darwin", "ios":
		files = append(files, system("Library", "Application Support", "pip", base))
	default:
		directories := m.Environment("XDG_CONFIG_DIRS")
		if directories == "" {
			files = append(files, system("etc", "xdg", "pip", base))
		}
		for _, d := range filepath.SplitList(directories) {
			files = append(files, join(d, "pip", base))
		}
		files = append(files, system("etc", base))
	}
	if environment == "" || !isFile(environment) {
		legacy := ".pip"
		if m.GOOS == "windows" {
			legacy = "pip"
		}
		files = append(files, join(m.Home, legacy, base))
		var user string
		switch m.GOOS {
		case "windows":
			user = join(m.Environment("APPDATA"), "pip")
		case "darwin", "ios":
			// pip keeps to ~/.config/pip on macOS until Application Support/pip exists.
			if user = join(m.Home, "Library", "Application Support", "pip"); !isDirectory(user) {
				user = join(m.Home, ".config", "pip")
			}
		default:
			user = join(m.xdgConfigHome(), "pip")
		}
		files = append(files, join(user, base))
	}
	if environment != "" {
		files = append(files, environment)
	}
	return files, true
}

// ---------------------------------------------------------------- containers

// ContainerAuthFiles are the registry credential files, the first one to hold a
// registry's credential being the one used, as containers-auth.json(5) orders them:
// REGISTRY_AUTH_FILE alone when it is set; else Podman's own
// ($XDG_RUNTIME_DIR/containers/auth.json on Linux, ~/.config/containers/auth.json
// elsewhere), then $XDG_CONFIG_HOME/containers/auth.json, then Docker's
// ($DOCKER_CONFIG/config.json, else ~/.docker/config.json).
//
// Implements: REQ-AUTH-020
func (m Machine) ContainerAuthFiles() []string {
	if f := m.Environment("REGISTRY_AUTH_FILE"); f != "" {
		return []string{f}
	}
	var files []string
	add := func(f string) {
		if f != "" && !contains(files, f) {
			files = append(files, f)
		}
	}
	if m.GOOS == "linux" {
		add(join(m.Environment("XDG_RUNTIME_DIR"), "containers", "auth.json"))
	} else {
		add(join(m.Home, ".config", "containers", "auth.json"))
	}
	add(join(m.xdgConfigHome(), "containers", "auth.json"))
	if directory := m.Environment("DOCKER_CONFIG"); directory != "" {
		add(filepath.Join(directory, "config.json"))
	} else {
		add(join(m.Home, ".docker", "config.json"))
	}
	return files
}

func contains(list []string, s string) bool {
	for _, have := range list {
		if have == s {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- netrc

// Netrc is the netrc file, as the go command (1.24 on) and curl find it: $NETRC; else
// on Windows ~/_netrc when it exists; else ~/.netrc.
//
// Implements: REQ-AUTH-002
func (m Machine) Netrc() string {
	if f := m.Environment("NETRC"); f != "" {
		return f
	}
	if m.GOOS == "windows" {
		if legacy := join(m.Home, "_netrc"); legacy != "" && isFile(legacy) {
			return legacy
		}
	}
	return join(m.Home, ".netrc")
}

// ---------------------------------------------------------------- Go

// GoEnvironmentFile is the file `go env -w` writes: $GOENV, none when that is "off", else
// go/env in the user config directory.
//
// Implements: REQ-SUP-064
func (m Machine) GoEnvironmentFile() string {
	switch f := m.Environment("GOENV"); f {
	case "off":
		return ""
	case "":
		return join(m.ConfigDirectory(), "go", "env")
	default:
		return f
	}
}

// GoEnvironment is a Go setting as the go command reads it: the process environment when the
// variable is set and not empty, else the go environment file.
//
// Implements: REQ-SUP-064, REQ-SUP-036
func (m Machine) GoEnvironment(key string) string {
	if v := m.Environment(key); v != "" {
		return v
	}
	f := m.GoEnvironmentFile()
	if f == "" {
		return ""
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return ""
	}
	value := ""
	for _, line := range strings.Split(string(data), "\n") {
		// The go command's own reading: a line is KEY=VALUE, KEY starting A-Z;
		// anything else is skipped, and a later line wins.
		k, v, ok := strings.Cut(strings.TrimSuffix(line, "\r"), "=")
		if ok && k == key {
			value = v
		}
	}
	return value
}

// ---------------------------------------------------------------- NuGet

// NuGetConfigs are the user's NuGet.Config files: %APPDATA%\NuGet\NuGet.Config on
// Windows; elsewhere (and on a Windows without %APPDATA%)
// ~/.nuget/NuGet/NuGet.Config, and ~/.config/NuGet/NuGet.Config where older Mono
// tooling keeps it.
//
// Implements: REQ-SUP-064
func (m Machine) NuGetConfigs() []string {
	if f := join(m.Environment("APPDATA"), "NuGet", "NuGet.Config"); f != "" && m.GOOS == "windows" {
		return []string{f}
	}
	if m.Home == "" {
		return nil
	}
	return []string{
		filepath.Join(m.Home, ".nuget", "NuGet", "NuGet.Config"),
		filepath.Join(m.Home, ".config", "NuGet", "NuGet.Config"),
	}
}

// NuGetMachineConfigs are NuGet's machine-wide configuration files, farther than the
// user's: every *.config in %ProgramFiles(x86)%\NuGet\Config on Windows (the
// 64-bit %ProgramFiles% without it); elsewhere (and on a Windows with neither set)
// in NuGet/Config under NUGET_COMMON_APPLICATION_DATA, else /Library/Application
// Support on macOS and /etc/opt on Linux. They are listed in name order.
//
// Implements: REQ-SUP-064
func (m Machine) NuGetMachineConfigs() []string {
	programFiles := m.Environment("ProgramFiles(x86)")
	if programFiles == "" {
		programFiles = m.Environment("ProgramFiles")
	}
	var directory string
	switch {
	case m.GOOS == "windows" && programFiles != "":
		directory = join(programFiles, "NuGet", "Config")
	case m.Environment("NUGET_COMMON_APPLICATION_DATA") != "":
		directory = join(m.Environment("NUGET_COMMON_APPLICATION_DATA"), "NuGet", "Config")
	case m.GOOS == "darwin":
		directory = system("Library", "Application Support", "NuGet", "Config")
	default:
		directory = system("etc", "opt", "NuGet", "Config")
	}
	if directory == "" {
		return nil
	}
	entries, _ := os.ReadDir(directory)
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".config") {
			out = append(out, filepath.Join(directory, e.Name()))
		}
	}
	return out
}

// nugetCredentialPrefix begins the variables NuGet reads a source's credential from,
// NuGetPackageSourceCredentials_<source name>.
const nugetCredentialPrefix = "NuGetPackageSourceCredentials_"

// NuGetCredentialVariables are the NuGetPackageSourceCredentials_<source> variables, by
// source name lower-cased. NuGet appends the source name as it is; the prefix and the
// name are matched without regard to case, as on Windows, and of two variables for
// one source the first in name order is taken.
//
// Implements: REQ-AUTH-022
func (m Machine) NuGetCredentialVariables() map[string]string {
	out := map[string]string{}
	n := len(nugetCredentialPrefix)
	for _, name := range m.names(func(s string) bool { return len(s) > n && strings.EqualFold(s[:n], nugetCredentialPrefix) }) {
		key := strings.ToLower(name[n:])
		if _, done := out[key]; done {
			continue
		}
		if v := m.Environment(name); v != "" {
			out[key] = v
		}
	}
	return out
}

// ---------------------------------------------------------------- Composer

// ComposerHome is the one directory Composer takes for its home: COMPOSER_HOME when
// set; else on Windows %APPDATA%\Composer (when %APPDATA% is set); else the first
// that exists of $XDG_CONFIG_HOME/composer (~/.config/composer without it) and
// ~/.composer.
//
// Implements: REQ-SUP-064, REQ-AUTH-016
func (m Machine) ComposerHome() string {
	if directory := m.Environment("COMPOSER_HOME"); directory != "" {
		return directory
	}
	if directory := m.Environment("APPDATA"); directory != "" && m.GOOS == "windows" {
		return filepath.Join(directory, "Composer")
	}
	for _, directory := range []string{join(m.xdgConfigHome(), "composer"), join(m.Home, ".composer")} {
		if directory != "" && isDirectory(directory) {
			return directory
		}
	}
	return ""
}

// ---------------------------------------------------------------- Bundler

// BundlerConfig is the user's Bundler config file, as Bundler finds it:
// BUNDLE_USER_CONFIG, else config in BUNDLE_USER_HOME, else ~/.bundle/config.
//
// Implements: REQ-AUTH-018, REQ-SUP-015
func (m Machine) BundlerConfig() string {
	if f := m.Environment("BUNDLE_USER_CONFIG"); f != "" {
		return f
	}
	if directory := m.Environment("BUNDLE_USER_HOME"); directory != "" {
		return filepath.Join(directory, "config")
	}
	return join(m.Home, ".bundle", "config")
}
