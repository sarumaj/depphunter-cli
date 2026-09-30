// Package userconf finds the files the package managers of this machine keep their
// configuration in, the way each of them finds its own: a variable naming the file or
// the directory first, then the platform's place for it.
//
// Index discovery (internal/index) and the credential store (internal/auth) both go
// through here, so the feed a configuration names and the credential kept beside it
// are always read from the same file. Nothing here reads the repository.
package userconf

import (
	"cmp"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	// Directory is the directory being analyzed, "" for none. The configuration
	// files some tools look for in every directory above a project are this
	// machine's in the directories above it that lie outside its checkout (see
	// DirectoriesAbove).
	Directory string
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

// isRegularFile is isFile without devices: a null device or a named pipe exists, but
// is no file a tool reads its configuration from.
func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// absolute reports whether p is absolute on this machine's platform rather than the
// one running depphunter: /x on Unix, and on Windows a drive or UNC path or one rooted
// on the current drive, none of which is taken from the working directory.
func (m Machine) absolute(p string) bool {
	if m.GOOS == "windows" {
		return windowsAbsolute(p) || strings.HasPrefix(p, `\`) || strings.HasPrefix(p, "/")
	}
	return strings.HasPrefix(p, "/")
}

// list splits a path-list variable with this machine's separator: ";" on Windows,
// ":" elsewhere.
func (m Machine) list(value string) []string {
	if value == "" {
		return nil
	}
	if m.GOOS == "windows" {
		return strings.Split(value, ";")
	}
	return strings.Split(value, ":")
}

// A location rule is a cmp.Or of its candidates in the tool's order - a variable, a
// path under a variable (under gives "" when the variable is unset), a platform's
// default, the default under the home directory - with the helpers below.

// firstFile is the first of paths that is a file, "" when none is.
func firstFile(paths ...string) string {
	for _, path := range paths {
		if path != "" && isFile(path) {
			return path
		}
	}
	return ""
}

// firstDirectory is the first of paths that is a directory, "" when none is.
func firstDirectory(paths ...string) string {
	for _, path := range paths {
		if path != "" && isDirectory(path) {
			return path
		}
	}
	return ""
}

// on is path on the platforms named, "" on the others.
func (m Machine) on(path string, platforms ...string) string {
	if slices.Contains(platforms, m.GOOS) {
		return path
	}
	return ""
}

// byPlatform is windows on Windows, falling back to other when it is "" (a
// known folder that is unset), macOS on macOS, even when it is "", and other
// elsewhere.
func (m Machine) byPlatform(windows, macOS, other string) string {
	switch m.GOOS {
	case "windows":
		return cmp.Or(windows, other)
	case "darwin", "ios":
		return macOS
	}
	return other
}

// home is a path under the home directory.
func (m Machine) home(element ...string) string { return join(m.Home, element...) }

// under is a path under the directory a variable names.
func (m Machine) under(variable string, element ...string) string {
	return join(m.Environment(variable), element...)
}

// applicationSupport is a path under macOS's ~/Library/Application Support.
func (m Machine) applicationSupport(element ...string) string {
	return m.home(append([]string{"Library", "Application Support"}, element...)...)
}

// present is paths without the "" ones, nil when none is left.
func present(paths ...string) []string {
	var out []string
	for _, path := range paths {
		if path != "" {
			out = append(out, path)
		}
	}
	return out
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
// ~/Library/Application Support on macOS, else $XDG_CONFIG_HOME or ~/.config.
//
// Implements: REQ-SUP-064
func (m Machine) ConfigDirectory() string {
	switch m.GOOS {
	case "windows":
		return m.Environment("APPDATA")
	case "darwin", "ios":
		return m.applicationSupport()
	}
	return m.xdgConfigHome()
}

// xdg is a path under the directory an XDG_* variable names, "" unless that is
// absolute: the XDG Base Directory Specification has a relative path ignored, and
// one would be taken from the working directory, often the repository analyzed.
func (m Machine) xdg(variable string, element ...string) string {
	if directory := m.Environment(variable); m.absolute(directory) {
		return join(directory, element...)
	}
	return ""
}

// xdgConfigHome is a path under $XDG_CONFIG_HOME, else under ~/.config.
func (m Machine) xdgConfigHome(element ...string) string {
	return join(cmp.Or(m.xdg("XDG_CONFIG_HOME"), m.home(".config")), element...)
}

// xdgConfigDirectories are the absolute directories of $XDG_CONFIG_DIRS (see xdg).
func (m Machine) xdgConfigDirectories() []string {
	var out []string
	for _, directory := range m.list(m.Environment("XDG_CONFIG_DIRS")) {
		if m.absolute(directory) {
			out = append(out, directory)
		}
	}
	return out
}

// ---------------------------------------------------------------- Cargo

// CargoHome is $CARGO_HOME, else ~/.cargo.
//
// Implements: REQ-SUP-064
func (m Machine) CargoHome() string {
	return cmp.Or(m.Environment("CARGO_HOME"), m.home(".cargo"))
}

// CargoFile is the file Cargo reads for name ("config", "credentials") in its home:
// the one without an extension when it exists - Cargo warns and uses it when both
// do - else name.toml.
//
// Implements: REQ-SUP-064
func (m Machine) CargoFile(name string) string {
	directory := m.CargoHome()
	return cmp.Or(firstFile(join(directory, name)), join(directory, name+".toml"))
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
	return cmp.Or(m.NpmEnvironment()["userconfig"], m.home(".npmrc"))
}

// NpmGlobalConfig is npm's global npmrc: the globalconfig setting of the environment,
// else etc/npmrc under the prefix the environment sets. npm's own default prefix is
// where node is installed, which is not guessed here: "" then.
//
// Implements: REQ-SUP-064
func (m Machine) NpmGlobalConfig() string {
	environment := m.NpmEnvironment()
	return cmp.Or(environment["globalconfig"], join(environment["prefix"], "etc", "npmrc"))
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

// YarnUserConfig is Yarn Berry's configuration in the home directory, which Yarn
// reads as .yarnrc.yml whatever name YARN_RC_FILENAME gives the files of the
// directories above a project.
//
// Implements: REQ-SUP-064
func (m Machine) YarnUserConfig() string { return m.home(".yarnrc.yml") }

// YarnConfigs are this machine's Yarn Berry configuration files, closest first, as
// Yarn finds them for a project in Directory: the file YARN_RC_FILENAME names (else
// .yarnrc.yml) in each directory above Directory that lies outside its checkout
// (DirectoriesAbove), then the home directory's .yarnrc.yml, which Yarn reads last
// when the walk did not already reach it. Yarn merges them key by key, the closest
// file winning.
//
// Implements: REQ-SUP-064, REQ-SUP-079
func (m Machine) YarnConfigs() []string {
	_, above := m.DirectoriesAbove()
	var out []string
	for _, directory := range above {
		if name := filepath.Join(directory, m.YarnRCFilename()); isFile(name) {
			out = append(out, name)
		}
	}
	if home := m.YarnUserConfig(); home != "" && !slices.Contains(out, home) {
		out = append(out, home)
	}
	return out
}

// YarnClassicConfig is Yarn 1's ~/.yarnrc.
//
// Implements: REQ-SUP-064
func (m Machine) YarnClassicConfig() string { return m.home(".yarnrc") }

// BunConfig is Bun's global bunfig: $XDG_CONFIG_HOME/.bunfig.toml when that
// exists, else ~/.bunfig.toml.
//
// Implements: REQ-SUP-064
func (m Machine) BunConfig() string {
	return cmp.Or(firstFile(m.xdg("XDG_CONFIG_HOME", ".bunfig.toml")), m.home(".bunfig.toml"))
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
		files = append(files, m.under("ProgramData", "pip", base))
	case "darwin", "ios":
		files = append(files, system("Library", "Application Support", "pip", base))
	default:
		directories := m.xdgConfigDirectories()
		if directories == nil {
			files = append(files, system("etc", "xdg", "pip", base))
		}
		for _, d := range directories {
			files = append(files, join(d, "pip", base))
		}
		files = append(files, system("etc", base))
	}
	if environment == "" || !isRegularFile(environment) {
		legacy := ".pip"
		if m.GOOS == "windows" {
			legacy = "pip"
		}
		files = append(files, m.home(legacy, base))
		var user string
		switch m.GOOS {
		case "windows":
			user = m.under("APPDATA", "pip")
		case "darwin", "ios":
			// pip keeps to ~/.config/pip on macOS until Application Support/pip exists.
			user = cmp.Or(firstDirectory(m.applicationSupport("pip")), m.home(".config", "pip"))
		default:
			user = m.xdgConfigHome("pip")
		}
		files = append(files, join(user, base))
	}
	return present(append(files, environment)...), true
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
		if f != "" && !slices.Contains(files, f) {
			files = append(files, f)
		}
	}
	if m.GOOS == "linux" {
		add(m.xdg("XDG_RUNTIME_DIR", "containers", "auth.json"))
	} else {
		add(m.home(".config", "containers", "auth.json"))
	}
	add(m.xdgConfigHome("containers", "auth.json"))
	add(cmp.Or(m.under("DOCKER_CONFIG", "config.json"), m.home(".docker", "config.json")))
	return files
}

// ---------------------------------------------------------------- netrc

// Netrc is the netrc file, as the go command (1.24 on) and curl find it: $NETRC; else
// on Windows ~/_netrc when it exists; else ~/.netrc.
//
// Implements: REQ-AUTH-002
func (m Machine) Netrc() string {
	return cmp.Or(m.Environment("NETRC"), m.on(firstFile(m.home("_netrc")), "windows"), m.home(".netrc"))
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
// tooling keeps it. They are followed by NuGet's additional user files, every
// *.config (and elsewhere than on Windows *.Config) in the config directory beside
// the user's NuGet.Config, other than a NuGet.Config, in name order - after the
// user's file, as NuGet reads them.
//
// Implements: REQ-SUP-064
func (m Machine) NuGetConfigs() []string {
	var out []string
	directory := m.under("APPDATA", "NuGet")
	switch {
	case directory != "" && m.GOOS == "windows":
		out = []string{filepath.Join(directory, "NuGet.Config")}
	case m.Home == "":
		return nil
	default:
		directory = filepath.Join(m.Home, ".nuget", "NuGet")
		out = []string{filepath.Join(directory, "NuGet.Config"), filepath.Join(m.Home, ".config", "NuGet", "NuGet.Config")}
	}
	return append(out, m.nugetAdditionalConfigs(filepath.Join(directory, "config"))...)
}

// nugetAdditionalConfigs are the additional user files of NuGet in directory: on
// Windows every *.config without regard to case, elsewhere *.config and *.Config;
// never a NuGet.Config; sorted the way NuGet sorts them (ordinally, on Windows
// without regard to case).
func (m Machine) nugetAdditionalConfigs(directory string) []string {
	entries, _ := os.ReadDir(directory)
	windows := m.GOOS == "windows"
	var out []string
	for _, e := range entries {
		name := e.Name()
		extension := filepath.Ext(name)
		switch {
		case e.IsDir():
		case windows && (!strings.EqualFold(extension, ".config") || strings.EqualFold(name, "NuGet.Config")):
		case !windows && (extension != ".config" && extension != ".Config" || name == "NuGet.Config"):
		default:
			out = append(out, name)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if windows {
			return strings.ToUpper(out[i]) < strings.ToUpper(out[j])
		}
		return out[i] < out[j]
	})
	for i, name := range out {
		out[i] = filepath.Join(directory, name)
	}
	return out
}

// NuGetConfigIn is the nuget.config NuGet reads in a directory it walks through,
// "" when there is none: on Windows NuGet.Config in any case; elsewhere the first
// that exists of nuget.config, NuGet.config and NuGet.Config.
//
// Implements: REQ-SUP-079
func (m Machine) NuGetConfigIn(directory string) string {
	if m.GOOS == "windows" {
		return firstFile(filepath.Join(directory, "NuGet.Config"))
	}
	return firstFile(filepath.Join(directory, "nuget.config"), filepath.Join(directory, "NuGet.config"),
		filepath.Join(directory, "NuGet.Config"))
}

// NuGetConfigsAbove are the nuget.config files of the directories above Directory
// that lie outside its checkout (DirectoriesAbove), closest first: NuGet reads them
// after a project's own and before the user's NuGet.Config.
//
// Implements: REQ-SUP-079
func (m Machine) NuGetConfigsAbove() []string {
	_, above := m.DirectoriesAbove()
	var out []string
	for _, directory := range above {
		if name := m.NuGetConfigIn(directory); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// NuGetMachineConfigs are NuGet's machine-wide configuration files, farther than the
// user's: every *.config in %ProgramFiles(x86)%\NuGet\Config on Windows (the
// 64-bit %ProgramFiles% without it); elsewhere (and on a Windows with neither set)
// in NuGet/Config under NUGET_COMMON_APPLICATION_DATA, else /Library/Application
// Support on macOS and /etc/opt on Linux. They are listed in name order.
//
// Implements: REQ-SUP-064
func (m Machine) NuGetMachineConfigs() []string {
	programFiles := cmp.Or(m.Environment("ProgramFiles(x86)"), m.Environment("ProgramFiles"))
	var directory string
	switch {
	case m.GOOS == "windows" && programFiles != "":
		directory = join(programFiles, "NuGet", "Config")
	case m.Environment("NUGET_COMMON_APPLICATION_DATA") != "":
		directory = m.under("NUGET_COMMON_APPLICATION_DATA", "NuGet", "Config")
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
	return cmp.Or(m.Environment("COMPOSER_HOME"), m.on(m.under("APPDATA", "Composer"), "windows"),
		firstDirectory(m.xdgConfigHome("composer"), m.home(".composer")))
}

// ---------------------------------------------------------------- Bundler

// BundlerConfig is the user's Bundler config file, as Bundler finds it:
// BUNDLE_USER_CONFIG, else config in BUNDLE_USER_HOME, else ~/.bundle/config.
//
// Implements: REQ-AUTH-018, REQ-SUP-015
func (m Machine) BundlerConfig() string {
	return cmp.Or(m.Environment("BUNDLE_USER_CONFIG"), m.under("BUNDLE_USER_HOME", "config"),
		m.home(".bundle", "config"))
}
