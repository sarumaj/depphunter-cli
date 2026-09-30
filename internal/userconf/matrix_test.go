package userconf

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The location matrix pins where every location function of this package looks, on
// every platform, for every environment that moves it and every file whose existence
// it asks about. testdata/locations.txt holds the answers; `go test -run
// TestLocationMatrix -update` rewrites it from the code as it is.
//
// In the file, "~" is the home directory, "$NAME" the directory the variable NAME is
// set to, "<system>" SystemRoot and "<root>" the test's directory; backslashes are
// written as slashes, so that one file holds for every platform the test runs on.
// Every case and every answer is a Go string literal, so that no line begins or ends
// in whitespace that means something; the indentation is only for reading.

var update = flag.Bool("update", false, "rewrite testdata/locations.txt")

var matrixPlatforms = []string{"linux", "freebsd", "darwin", "ios", "windows"}

// The platform variables: the XDG base directories and Windows' known folders.
var (
	xdgVariables     = []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_CONFIG_DIRS", "XDG_RUNTIME_DIR"}
	appDataVariables = []string{"APPDATA", "LOCALAPPDATA", "ProgramData", "ProgramFiles", "ProgramFiles(x86)"}
)

// locationRule is one location function as the matrix calls it.
type locationRule struct {
	name string
	// variables are the tool's own variables, each set to a directory of its own.
	variables []string
	// values are further values a variable is tried with, alone.
	values map[string][]string
	// layouts are the file systems tried beside the empty one: the files (a trailing
	// "/" makes a directory) to create below the test's directory.
	layouts map[string][]string
	call    func(m Machine, root string) string
}

func show(paths []string) string {
	if paths == nil {
		return "[]"
	}
	return "[" + strings.Join(paths, ", ") + "]"
}

// outsideCheckout keeps, of paths, those below root: the directories above root
// are the machine's that runs the test.
func outsideCheckout(paths []string, root string) []string {
	var out []string
	for _, p := range paths {
		if strings.HasPrefix(p, root+string(filepath.Separator)) {
			out = append(out, p)
		}
	}
	return out
}

var locationRules = []locationRule{
	{name: "ConfigDirectory", call: func(m Machine, _ string) string { return m.ConfigDirectory() }},
	{name: "CargoHome", variables: []string{"CARGO_HOME"}, call: func(m Machine, _ string) string { return m.CargoHome() }},
	{name: "CargoFile(config) CargoFile(credentials)", variables: []string{"CARGO_HOME"},
		layouts: map[string][]string{"legacy": {"home/.cargo/config", "home/.cargo/credentials/", "env/CARGO_HOME/config", "env/CARGO_HOME/credentials/"}},
		call:    func(m Machine, _ string) string { return m.CargoFile("config") + " " + m.CargoFile("credentials") }},
	{name: "NpmUserConfig", variables: []string{"NPM_CONFIG_USERCONFIG", "npm_config_userconfig"},
		call: func(m Machine, _ string) string { return m.NpmUserConfig() }},
	{name: "NpmGlobalConfig", variables: []string{"npm_config_globalconfig", "NPM_CONFIG_GLOBALCONFIG", "npm_config_prefix", "NPM_CONFIG_PREFIX"},
		call: func(m Machine, _ string) string { return m.NpmGlobalConfig() }},
	{name: "YarnRCFilename", variables: []string{"YARN_RC_FILENAME"}, values: map[string][]string{"YARN_RC_FILENAME": {"custom.yml", "a/b.yml", `a\b.yml`}},
		call: func(m Machine, _ string) string { return m.YarnRCFilename() }},
	{name: "YarnUserConfig", call: func(m Machine, _ string) string { return m.YarnUserConfig() }},
	{name: "YarnClassicConfig", call: func(m Machine, _ string) string { return m.YarnClassicConfig() }},
	{name: "YarnConfigs", variables: []string{"YARN_RC_FILENAME"}, values: map[string][]string{"YARN_RC_FILENAME": {"custom.yml"}},
		layouts: map[string][]string{"above": {"work/checkout/.git/", "work/.yarnrc.yml", "work/custom.yml", ".yarnrc.yml", "work/checkout/.yarnrc.yml", "home/.yarnrc.yml"}},
		call: func(m Machine, root string) string {
			m.Directory = filepath.Join(root, "work", "checkout", "project")
			return show(outsideCheckout(m.YarnConfigs(), root))
		}},
	{name: "BunConfig", layouts: map[string][]string{"xdg": {"env/XDG_CONFIG_HOME/.bunfig.toml"}},
		call: func(m Machine, _ string) string { return m.BunConfig() }},
	{name: "PipConfigFiles", variables: []string{"PIP_CONFIG_FILE"},
		values:  map[string][]string{"PIP_CONFIG_FILE": {"/dev/null", "nul", "NUL"}},
		layouts: map[string][]string{"files": {"env/PIP_CONFIG_FILE", "home/Library/Application Support/pip/"}},
		call: func(m Machine, _ string) string {
			files, ok := m.PipConfigFiles()
			return fmt.Sprintf("%s ok=%v", show(files), ok)
		}},
	{name: "ContainerAuthFiles", variables: []string{"REGISTRY_AUTH_FILE", "DOCKER_CONFIG"},
		call: func(m Machine, _ string) string { return show(m.ContainerAuthFiles()) }},
	{name: "Netrc", variables: []string{"NETRC"}, layouts: map[string][]string{"_netrc": {"home/_netrc"}, "_netrc directory": {"home/_netrc/"}},
		call: func(m Machine, _ string) string { return m.Netrc() }},
	{name: "GoEnvironmentFile", variables: []string{"GOENV"}, values: map[string][]string{"GOENV": {"off"}},
		call: func(m Machine, _ string) string { return m.GoEnvironmentFile() }},
	{name: "NuGetConfigs", layouts: map[string][]string{"additional": {
		"home/.nuget/NuGet/config/b.config", "home/.nuget/NuGet/config/A.Config", "home/.nuget/NuGet/config/c.CONFIG",
		"home/.nuget/NuGet/config/NuGet.Config", "home/.nuget/NuGet/config/d.txt", "home/.nuget/NuGet/config/e.config/",
		"env/APPDATA/NuGet/config/b.config", "env/APPDATA/NuGet/config/A.Config", "env/APPDATA/NuGet/config/c.CONFIG",
		"env/APPDATA/NuGet/config/NUGET.CONFIG", "env/APPDATA/NuGet/config/d.txt",
	}}, call: func(m Machine, _ string) string { return show(m.NuGetConfigs()) }},
	{name: "NuGetConfigIn", layouts: map[string][]string{"all spellings": {"work/nuget.config", "work/NuGet.config", "work/NuGet.Config"}},
		call: func(m Machine, root string) string { return m.NuGetConfigIn(filepath.Join(root, "work")) }},
	{name: "NuGetConfigsAbove", layouts: map[string][]string{"above": {"work/checkout/.git/", "work/NuGet.Config", "work/nuget.config", "work/NuGet.config", "work/checkout/NuGet.Config"}},
		call: func(m Machine, root string) string {
			m.Directory = filepath.Join(root, "work", "checkout", "project")
			return show(outsideCheckout(m.NuGetConfigsAbove(), root))
		}},
	{name: "NuGetMachineConfigs", variables: []string{"ProgramFiles", "ProgramFiles(x86)", "NUGET_COMMON_APPLICATION_DATA"},
		layouts: map[string][]string{"files": {
			"env/ProgramFiles/NuGet/Config/a.config", "env/ProgramFiles(x86)/NuGet/Config/b.Config",
			"env/NUGET_COMMON_APPLICATION_DATA/NuGet/Config/c.CONFIG", "env/NUGET_COMMON_APPLICATION_DATA/NuGet/Config/d.txt",
			"system/Library/Application Support/NuGet/Config/e.config", "system/etc/opt/NuGet/Config/f.config",
			"system/etc/opt/NuGet/Config/g.config/",
		}}, call: func(m Machine, _ string) string { return show(m.NuGetMachineConfigs()) }},
	{name: "ComposerHome", variables: []string{"COMPOSER_HOME"},
		layouts: map[string][]string{
			"both":   {"env/XDG_CONFIG_HOME/composer/", "home/.config/composer/", "home/.composer/"},
			"legacy": {"home/.composer/"},
			"files":  {"env/XDG_CONFIG_HOME/composer", "home/.config/composer", "home/.composer"},
		}, call: func(m Machine, _ string) string { return m.ComposerHome() }},
	{name: "BundlerConfig", variables: []string{"BUNDLE_USER_CONFIG", "BUNDLE_USER_HOME"},
		call: func(m Machine, _ string) string { return m.BundlerConfig() }},
	{name: "RegistriesConf", variables: []string{"CONTAINERS_REGISTRIES_CONF"},
		layouts: map[string][]string{
			"drop-ins": {"system/etc/containers/registries.conf.d/b.conf", "system/etc/containers/registries.conf.d/a.conf",
				"system/etc/containers/registries.conf.d/c.txt", "home/.config/containers/registries.conf.d/d.conf",
				"env/XDG_CONFIG_HOME/containers/registries.conf.d/e.conf", "env/XDG_CONFIG_HOME/containers/registries.conf.d/f.conf/"},
			"user file": {"home/.config/containers/registries.conf", "env/XDG_CONFIG_HOME/containers/registries.conf",
				"system/etc/containers/registries.conf.d/a.conf", "home/.config/containers/registries.conf.d/d.conf",
				"env/XDG_CONFIG_HOME/containers/registries.conf.d/e.conf"},
		}, call: func(m Machine, _ string) string {
			main, dropIns := m.RegistriesConf()
			return main + " " + show(dropIns)
		}},
	{name: "DockerDaemonConfig",
		layouts: map[string][]string{
			"all": {"home/.docker/daemon.json", "env/ProgramData/docker/config/daemon.json", "env/XDG_CONFIG_HOME/docker/daemon.json",
				"home/.config/docker/daemon.json", "system/etc/docker/daemon.json"},
			"system": {"env/ProgramData/docker/config/daemon.json", "system/etc/docker/daemon.json"},
		}, call: func(m Machine, _ string) string { return m.DockerDaemonConfig() }},
	{name: "GitHubCLIConfigDirectory", variables: []string{"GH_CONFIG_DIR"},
		call: func(m Machine, _ string) string { return m.GitHubCLIConfigDirectory() }},
	{name: "PSResourceGetRepositories",
		layouts: map[string][]string{"macOS": {"home/Library/Application Support/PSResourceGet/PSResourceRepository.xml"}},
		call:    func(m Machine, _ string) string { return m.PSResourceGetRepositories() }},
	{name: "PowerShellGetRepositories", call: func(m Machine, _ string) string { return m.PowerShellGetRepositories() }},
	{name: "DartConfigDirectory", call: func(m Machine, _ string) string { return m.DartConfigDirectory() }},
	{name: "PubTokens", call: func(m Machine, _ string) string { return m.PubTokens() }},
	{name: "HexHome", variables: []string{"HEX_HOME", "MIX_XDG"}, values: map[string][]string{"MIX_XDG": {"1", "true", "yes", "TRUE"}},
		call: func(m Machine, _ string) string { return m.HexHome() }},
	{name: "Rebar3GlobalConfig", variables: []string{"REBAR_GLOBAL_CONFIG_DIR", "REBAR_CACHE_DIR"},
		call: func(m Machine, _ string) string { return m.Rebar3GlobalConfig() }},
	{name: "Rebar3HexConfig", variables: []string{"REBAR_GLOBAL_CONFIG_DIR", "REBAR_CACHE_DIR"},
		call: func(m Machine, _ string) string { return m.Rebar3HexConfig() }},
	{name: "MavenSettings", variables: []string{"MAVEN_HOME", "M2_HOME"},
		call: func(m Machine, _ string) string { return show(m.MavenSettings()) }},
	{name: "SbtRepositories", variables: []string{"JAVA_OPTS", "SBT_OPTS"},
		values: map[string][]string{
			"JAVA_OPTS": {"-Dsbt.repository.config=/r/repositories", "-Dsbt.global.base=/g", `-J-Dsbt.global.base="/q"`, "-Dsbt.global.base=", "-Dsbt.repository.config"},
			"SBT_OPTS":  {"-Dsbt.global.base=/s -Dsbt.global.base=/t"},
		}, call: func(m Machine, _ string) string { return m.SbtRepositories() }},
	{name: "SbtOverrideBuildRepositories", variables: []string{"SBT_OPTS"},
		values: map[string][]string{"SBT_OPTS": {"-Dsbt.override.build.repos=true", "-Dsbt.override.build.repos", "-Dsbt.override.build.repos=TRUE", "-Dsbt.override.build.repos=false"}},
		call:   func(m Machine, _ string) string { return fmt.Sprint(m.SbtOverrideBuildRepositories()) }},
	{name: "SbtCredentials", variables: []string{"SBT_CREDENTIALS", "JAVA_OPTS"},
		values: map[string][]string{"JAVA_OPTS": {"-Dsbt.global.base=/g"}},
		call:   func(m Machine, _ string) string { return show(m.SbtCredentials()) }},
	{name: "CoursierConfigDirectory", variables: []string{"COURSIER_CONFIG_DIR"},
		call: func(m Machine, _ string) string { return m.CoursierConfigDirectory() }},
	{name: "CoursierCredentials", variables: []string{"COURSIER_CREDENTIALS"},
		values: map[string][]string{"COURSIER_CREDENTIALS": {
			"host user:password", "file:///C:/x/c.properties", "file:/etc/c.properties", "file://server/share/c.properties",
			"FILE:///tmp/a%20b", "file://localhost/l.properties", "/absolute/c.properties", "relative/c.properties",
		}}, call: func(m Machine, _ string) string {
			inline, file := m.CoursierCredentials()
			return fmt.Sprintf("inline=%q file=%s", inline, file)
		}},
	{name: "GradleUserHome", variables: []string{"GRADLE_USER_HOME"}, call: func(m Machine, _ string) string { return m.GradleUserHome() }},
	{name: "GradleInitScripts", variables: []string{"GRADLE_USER_HOME"},
		layouts: map[string][]string{"scripts": {
			"home/.gradle/init.gradle", "home/.gradle/init.gradle.kts", "home/.gradle/init.d/b.gradle", "home/.gradle/init.d/a.gradle.kts",
			"home/.gradle/init.d/c.txt", "env/GRADLE_USER_HOME/init.gradle.kts", "env/GRADLE_USER_HOME/init.d/z.gradle",
		}}, call: func(m Machine, _ string) string { return show(m.GradleInitScripts()) }},
	{name: "ClojureConfigDirectory", variables: []string{"CLJ_CONFIG"}, call: func(m Machine, _ string) string { return m.ClojureConfigDirectory() }},
	{name: "LeinProfiles", variables: []string{"LEIN_HOME"}, call: func(m Machine, _ string) string { return m.LeinProfiles() }},
	{name: "UVNoConfig UVProjectConfig", variables: []string{"UV_NO_CONFIG", "UV_CONFIG_FILE"},
		values: map[string][]string{"UV_NO_CONFIG": {"1", " TRUE ", "on", "y", "t", "0", "no"}},
		call:   func(m Machine, _ string) string { return fmt.Sprint(m.UVNoConfig(), " ", m.UVProjectConfig()) }},
	{name: "UVConfigFiles", variables: []string{"UV_CONFIG_FILE", "UV_NO_CONFIG"},
		layouts: map[string][]string{"system": {"env/XDG_CONFIG_DIRS/b/uv/uv.toml", "env/XDG_CONFIG_DIRS/c/uv/uv.toml"}},
		call:    func(m Machine, _ string) string { return show(m.UVConfigFiles()) }},
	{name: "PoetryConfigDirectory", variables: []string{"POETRY_CONFIG_DIR"}, call: func(m Machine, _ string) string { return m.PoetryConfigDirectory() }},
	{name: "PDMConfigFile", variables: []string{"PDM_CONFIG_FILE"}, call: func(m Machine, _ string) string { return m.PDMConfigFile() }},
	{name: "DubUserDirectory", variables: []string{"DUB_HOME", "DPATH"}, call: func(m Machine, _ string) string { return m.DubUserDirectory() }},
	{name: "DubSettings", variables: []string{"DUB_HOME", "DPATH"}, call: func(m Machine, _ string) string { return show(m.DubSettings()) }},
	{name: "QuicklispDists", layouts: map[string][]string{"dists": {"home/quicklisp/dists/b/distinfo.txt", "home/quicklisp/dists/a/distinfo.txt", "home/quicklisp/dists/c/"}},
		call: func(m Machine, _ string) string { return show(m.QuicklispDists()) }},
	{name: "OpamRoot", variables: []string{"OPAMROOT"}, call: func(m Machine, _ string) string { return m.OpamRoot() }},
	{name: "AlireSettingsDirectory", variables: []string{"ALIRE_SETTINGS_DIR", "ALR_CONFIG"}, call: func(m Machine, _ string) string { return m.AlireSettingsDirectory() }},
	{name: "JuliaDepots", variables: []string{"JULIA_DEPOT_PATH"},
		values: map[string][]string{"JULIA_DEPOT_PATH": {"/a:/b", "/a;/b", ":/a", "/a:", "~:~/d:~", `~\e;/f`, "/a:/a", "::"}},
		call:   func(m Machine, _ string) string { return show(m.JuliaDepots()) }},
	{name: "R10KConfigs", call: func(m Machine, _ string) string { return show(m.R10KConfigs()) }},
	{name: "CUEConfigDirectory", variables: []string{"CUE_CONFIG_DIR"}, call: func(m Machine, _ string) string { return m.CUEConfigDirectory() }},
	{name: "CocoaPodsRepositories", variables: []string{"CP_REPOS_DIR", "CP_HOME_DIR"}, call: func(m Machine, _ string) string { return m.CocoaPodsRepositories() }},
	{name: "SwiftPMRegistries", layouts: map[string][]string{"idiomatic": {"home/Library/org.swift.swiftpm/configuration/registries.json"}},
		call: func(m Machine, _ string) string { return m.SwiftPMRegistries() }},
	{name: "ConanHome", variables: []string{"CONAN_HOME"},
		values: map[string][]string{"CONAN_HOME": {"~", "~/conan", `~\conan`, "relative", " /spaced ", "/absolute", `\\server\share`, `C:\conan`}},
		call:   func(m Machine, _ string) string { return m.ConanHome() }},
	{name: "CabalConfig", variables: []string{"CABAL_CONFIG", "CABAL_DIR"},
		layouts: map[string][]string{
			"legacy":          {"home/.cabal/"},
			"legacy and xdg":  {"home/.cabal/", "home/.config/cabal/config", "env/XDG_CONFIG_HOME/cabal/config"},
			"legacy file":     {"home/.cabal"},
			"xdg config only": {"home/.config/cabal/config"},
		}, call: func(m Machine, _ string) string { return m.CabalConfig() }},
	{name: "LuaRocksConfigs", variables: []string{"LUAROCKS_CONFIG", "LUAROCKS_CONFIG_5_3"},
		layouts: map[string][]string{
			"files": {"env/LUAROCKS_CONFIG", "env/LUAROCKS_CONFIG_5_3", "home/.luarocks/config-5.1.lua", "home/.luarocks/config-5.2.lua",
				"env/XDG_CONFIG_HOME/luarocks/config-5.2.lua", "home/.config/luarocks/config-5.4.lua", "env/APPDATA/luarocks/config-5.3.lua"},
			"user only": {"home/.luarocks/config-5.1.lua", "home/.config/luarocks/config-5.1.lua", "env/APPDATA/luarocks/config-5.4.lua"},
		}, call: func(m Machine, _ string) string { return show(m.LuaRocksConfigs()) }},
}

// matrixCase is one environment of the matrix, applied on every platform and in
// every layout of a rule.
type matrixCase struct {
	name      string
	noHome    bool
	variables map[string]string
}

// matrixCases are a rule's environments: none; each platform group; the tool's
// variables one by one, alone and over the platform's; all of them; all set empty;
// and without a home directory.
func matrixCases(rule locationRule, root string) []matrixCase {
	directory := func(name string) string { return filepath.Join(root, "env", name) }
	set := func(variables map[string]string, names ...string) {
		for _, name := range names {
			variables[name] = directory(name)
		}
		if _, ok := variables["XDG_CONFIG_DIRS"]; ok && slices.Contains(names, "XDG_CONFIG_DIRS") {
			variables["XDG_CONFIG_DIRS"] = strings.Join([]string{
				filepath.Join(directory("XDG_CONFIG_DIRS"), "a"), filepath.Join(directory("XDG_CONFIG_DIRS"), "b"), filepath.Join(directory("XDG_CONFIG_DIRS"), "c"),
			}, string(os.PathListSeparator))
		}
	}
	with := func(names ...string) map[string]string {
		variables := map[string]string{}
		set(variables, names...)
		return variables
	}
	platform := slices.Concat(xdgVariables, appDataVariables)
	relative := map[string]string{}
	for _, name := range xdgVariables {
		relative[name] = "relative"
	}
	empty := map[string]string{}
	for _, name := range slices.Concat(platform, rule.variables) {
		empty[name] = ""
	}
	cases := []matrixCase{
		{name: "none", variables: map[string]string{}},
		{name: "empty", variables: empty},
		{name: "xdg", variables: with(xdgVariables...)},
		{name: "xdg-relative", variables: relative},
		{name: "appdata", variables: with(appDataVariables...)},
		{name: "APPDATA", variables: with("APPDATA")},
		{name: "LOCALAPPDATA", variables: with("LOCALAPPDATA")},
		{name: "xdg+appdata", variables: with(platform...)},
	}
	for _, name := range rule.variables {
		cases = append(cases, matrixCase{name: name, variables: with(name)})
		for _, value := range rule.values[name] {
			cases = append(cases, matrixCase{name: name + "=" + value, variables: map[string]string{name: value}})
		}
		cases = append(cases, matrixCase{name: name + "+xdg+appdata", variables: with(append(slices.Clone(platform), name)...)})
	}
	if len(rule.variables) > 1 {
		cases = append(cases, matrixCase{name: "all", variables: with(rule.variables...)})
	}
	if len(rule.variables) > 0 {
		cases = append(cases, matrixCase{name: "all+xdg+appdata", variables: with(slices.Concat(platform, rule.variables)...)})
	}
	cases = append(cases,
		matrixCase{name: "no-home", noHome: true, variables: map[string]string{}},
		matrixCase{name: "no-home+xdg+appdata", noHome: true, variables: with(platform...)},
	)
	if len(rule.variables) > 0 {
		cases = append(cases, matrixCase{name: "no-home+all", noHome: true, variables: with(rule.variables...)})
	}
	return cases
}

// windowsOnly reports whether a value is an absolute path only where the platform
// running the test is Windows (a drive letter): filepath.IsAbs answers for that
// platform, not for the machine's GOOS.
func windowsOnly(variables map[string]string) bool {
	for _, value := range variables {
		if len(value) >= 2 && value[1] == ':' && driveLetter(value[0]) || strings.HasPrefix(value, `\\`) {
			return true
		}
	}
	return false
}

// normalize writes a result in the file's placeholders, quoted.
func normalize(result, root string) string {
	result = strings.ReplaceAll(result, `\`, "/")
	root = strings.ReplaceAll(root, `\`, "/")
	for _, replacement := range [][2]string{{root + "/home", "~"}, {root + "/env/", "$"}, {root + "/system", "<system>"}, {root, "<root>"}} {
		result = strings.ReplaceAll(result, replacement[0], replacement[1])
	}
	return strconv.Quote(result)
}

// build creates a layout's files and directories below root.
func build(t *testing.T, root string, files []string) {
	t.Helper()
	for _, f := range files {
		path := filepath.Join(root, filepath.FromSlash(f))
		if strings.HasSuffix(f, "/") {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		touch(t, path)
	}
}

// parseLocations reads testdata/locations.txt back: the answers by rule and case.
func parseLocations(data string) map[string]string {
	out := map[string]string{}
	rule, label := "", ""
	for _, line := range strings.Split(data, "\n") {
		switch line = strings.TrimSpace(line); {
		case line == "":
		case strings.HasPrefix(line, "# "):
			rule = line[2:]
		case strings.HasPrefix(line, `"`):
			label, _ = strconv.Unquote(line)
		default:
			key := rule + " | " + label
			out[key] = strings.TrimPrefix(out[key]+"\n"+line, "\n")
		}
	}
	return out
}

// Every location function answers, on every platform, as testdata/locations.txt says.
//
// Verifies: REQ-SUP-064
func TestLocationMatrix(t *testing.T) {
	saved := SystemRoot
	t.Cleanup(func() { SystemRoot = saved })
	var out strings.Builder
	got := map[string]string{}
	hostDependent := map[string]bool{}
	cells := 0
	for i, rule := range locationRules {
		if i > 0 {
			out.WriteString("\n")
		}
		fmt.Fprintf(&out, "# %s\n", rule.name)
		layouts := []string{""}
		for name := range rule.layouts {
			layouts = append(layouts, name)
		}
		slices.Sort(layouts)
		for _, layout := range layouts {
			root := t.TempDir()
			SystemRoot = filepath.Join(root, "system")
			build(t, root, rule.layouts[layout])
			for _, testCase := range matrixCases(rule, root) {
				home := filepath.Join(root, "home")
				if testCase.noHome {
					home = ""
				}
				variables := testCase.variables
				environ := func() []string {
					var list []string
					for name, value := range variables {
						list = append(list, name+"="+value)
					}
					slices.Sort(list)
					return list
				}
				// The platforms, grouped by answer in matrixPlatforms' order.
				var platforms, answers []string
				for _, goos := range matrixPlatforms {
					m := Machine{Home: home, GOOS: goos, Environ: environ, Environment: func(name string) string { return variables[name] }}
					answer := normalize(rule.call(m, root), root)
					cells++
					if i := slices.Index(answers, answer); i >= 0 {
						platforms[i] += "," + goos
						continue
					}
					platforms, answers = append(platforms, goos), append(answers, answer)
				}
				label := testCase.name
				if layout != "" {
					label = "[" + layout + "] " + label
				}
				var block []string
				for i := range answers {
					block = append(block, platforms[i]+": "+answers[i])
				}
				key := rule.name + " | " + label
				got[key] = strings.Join(block, "\n")
				hostDependent[key] = runtime.GOOS == "windows" && windowsOnly(variables)
				fmt.Fprintf(&out, "%s\n  %s\n", strconv.Quote(label), strings.Join(block, "\n  "))
			}
		}
	}
	golden := filepath.Join("testdata", "locations.txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(out.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%d functions' rules, %d cases, %d cells", len(locationRules), len(got), cells)
		return
	}
	data, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	want := parseLocations(string(data))
	if len(want) != len(got) {
		t.Errorf("testdata/locations.txt has %d cases, the matrix %d: rerun with -update and review the difference", len(want), len(got))
	}
	for key, answer := range got {
		// A drive letter is an absolute path to filepath.IsAbs on a Windows host
		// whatever the machine's GOOS: those cases hold on the other hosts only.
		if hostDependent[key] {
			continue
		}
		if want[key] != answer {
			t.Errorf("%s:\n got %s\nwant %s", key, strings.ReplaceAll(answer, "\n", "\n     "), strings.ReplaceAll(want[key], "\n", "\n     "))
		}
	}
}
