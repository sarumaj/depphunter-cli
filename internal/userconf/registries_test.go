package userconf

import (
	"path/filepath"
	"reflect"
	"testing"
)

// dub's user settings are DUB_HOME's, DPATH/dub's, %APPDATA%\dub's on Windows (or
// ~/.dub without %APPDATA%), else ~/.dub's; the system's are %ProgramData%\dub's
// on Windows, else /etc/dub's and /var/lib/dub's.
//
// Verifies: REQ-SUP-064
func TestDubSettings(t *testing.T) {
	home, directory, appData, programData := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	for _, testCase := range []struct {
		goos      string
		variables map[string]string
		user      string
	}{
		{"linux", nil, filepath.Join(home, ".dub")},
		{"darwin", nil, filepath.Join(home, ".dub")},
		{"linux", map[string]string{"DUB_HOME": directory, "DPATH": appData}, directory},
		{"linux", map[string]string{"DPATH": directory}, filepath.Join(directory, "dub")},
		{"windows", map[string]string{"APPDATA": appData, "ProgramData": programData}, filepath.Join(appData, "dub")},
		{"windows", nil, filepath.Join(home, ".dub")},
	} {
		m := machine(t, home, testCase.goos, testCase.variables)
		want := []string{filepath.Join(testCase.user, "settings.json")}
		switch {
		case testCase.goos != "windows":
			want = append(want, filepath.Join(SystemRoot, "etc", "dub", "settings.json"), filepath.Join(SystemRoot, "var", "lib", "dub", "settings.json"))
		case testCase.variables["ProgramData"] != "":
			want = append(want, filepath.Join(programData, "dub", "settings.json"))
		}
		if got := m.DubSettings(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s %v:\n got %v\nwant %v", testCase.goos, testCase.variables, got, want)
		}
	}
}

// The dists installed in ~/quicklisp, by name.
//
// Verifies: REQ-SUP-064
func TestQuicklispDists(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"quicklisp/dists/ultralisp/distinfo.txt", "quicklisp/dists/quicklisp/distinfo.txt", "quicklisp/dists/quicklisp/releases.txt"} {
		touch(t, filepath.Join(home, filepath.FromSlash(name)))
	}
	want := []string{
		filepath.Join(home, "quicklisp", "dists", "quicklisp", "distinfo.txt"),
		filepath.Join(home, "quicklisp", "dists", "ultralisp", "distinfo.txt"),
	}
	if got := machine(t, home, "linux", nil).QuicklispDists(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := machine(t, "", "linux", nil).QuicklispDists(); got != nil {
		t.Errorf("no home: %v", got)
	}
}

// opam's root is OPAMROOT, else %LOCALAPPDATA%\opam on Windows, else ~/.opam;
// Alire's settings are ALIRE_SETTINGS_DIR's (ALR_CONFIG's for Alire 1), else
// %USERPROFILE%\.config\alire on Windows, else $XDG_CONFIG_HOME/alire
// (~/.config/alire).
//
// Verifies: REQ-SUP-064
func TestOpamAndAlireLocations(t *testing.T) {
	home, directory, local, xdg := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	for _, testCase := range []struct {
		goos        string
		variables   map[string]string
		opam, alire string
	}{
		{"linux", nil, filepath.Join(home, ".opam"), filepath.Join(home, ".config", "alire")},
		{"darwin", map[string]string{"XDG_CONFIG_HOME": xdg}, filepath.Join(home, ".opam"), filepath.Join(xdg, "alire")},
		{"linux", map[string]string{"OPAMROOT": directory, "ALIRE_SETTINGS_DIR": directory, "ALR_CONFIG": local}, directory, directory},
		{"linux", map[string]string{"ALR_CONFIG": local}, filepath.Join(home, ".opam"), local},
		{"windows", map[string]string{"LOCALAPPDATA": local, "XDG_CONFIG_HOME": xdg}, filepath.Join(local, "opam"), filepath.Join(home, ".config", "alire")},
		{"windows", nil, filepath.Join(home, ".opam"), filepath.Join(home, ".config", "alire")},
	} {
		m := machine(t, home, testCase.goos, testCase.variables)
		if got := m.OpamRoot(); got != testCase.opam {
			t.Errorf("%s %v: opam root %s, want %s", testCase.goos, testCase.variables, got, testCase.opam)
		}
		if got := m.AlireSettingsDirectory(); got != testCase.alire {
			t.Errorf("%s %v: alire settings %s, want %s", testCase.goos, testCase.variables, got, testCase.alire)
		}
	}
	if m := machine(t, "", "linux", nil); m.OpamRoot() != "" || m.AlireSettingsDirectory() != "" {
		t.Error("no home, no paths")
	}
}

// JULIA_DEPOT_PATH lists the depots, `;`-separated on Windows and
// `:`-separated elsewhere; an empty entry is the default depot, `~` the home
// directory, and a depot named twice is listed once. Unset, the default depot
// ~/.julia is the only one.
//
// Verifies: REQ-SUP-055
func TestJuliaDepots(t *testing.T) {
	home := t.TempDir()
	user := filepath.Join(home, ".julia")
	for _, testCase := range []struct {
		goos, value string
		want        []string
	}{
		{"linux", "", []string{user}},
		{"linux", "/a:/b", []string{"/a", "/b"}},
		{"linux", ":/a", []string{user, "/a"}},
		{"darwin", "/a::/b:", []string{"/a", user, "/b"}},
		{"linux", "~/depot:/a:/a", []string{filepath.Join(home, "depot"), "/a"}},
		{"windows", `C:\depot;;D:\other`, []string{`C:\depot`, user, `D:\other`}},
		{"windows", `C:\depot`, []string{`C:\depot`}},
		{"linux", "/a;/b", []string{"/a;/b"}},
	} {
		m := machine(t, home, testCase.goos, map[string]string{"JULIA_DEPOT_PATH": testCase.value})
		if got := m.JuliaDepots(); !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("%s %q: got %v, want %v", testCase.goos, testCase.value, got, testCase.want)
		}
	}
}

// CocoaPods' spec repositories are in CP_REPOS_DIR, else repos below
// CP_HOME_DIR, else ~/.cocoapods/repos. SwiftPM's user registries.json is in
// ~/Library/org.swift.swiftpm/configuration on macOS when it is there, else in
// configuration below $XDG_CONFIG_HOME/swiftpm or ~/.swiftpm.
//
// Verifies: REQ-SUP-064
func TestCocoaPodsAndSwiftPMLocations(t *testing.T) {
	home, directory, xdg := t.TempDir(), t.TempDir(), t.TempDir()
	for _, testCase := range []struct {
		variables map[string]string
		want      string
	}{
		{nil, filepath.Join(home, ".cocoapods", "repos")},
		{map[string]string{"CP_HOME_DIR": directory}, filepath.Join(directory, "repos")},
		{map[string]string{"CP_HOME_DIR": xdg, "CP_REPOS_DIR": directory}, directory},
	} {
		if got := machine(t, home, "linux", testCase.variables).CocoaPodsRepositories(); got != testCase.want {
			t.Errorf("%v: got %s, want %s", testCase.variables, got, testCase.want)
		}
	}
	dot := filepath.Join(home, ".swiftpm", "configuration", "registries.json")
	library := filepath.Join(home, "Library", "org.swift.swiftpm", "configuration", "registries.json")
	for _, testCase := range []struct {
		goos      string
		variables map[string]string
		want      string
	}{
		{"linux", nil, dot},
		{"windows", nil, dot},
		{"linux", map[string]string{"XDG_CONFIG_HOME": xdg}, filepath.Join(xdg, "swiftpm", "configuration", "registries.json")},
		{"darwin", nil, dot},
	} {
		if got := machine(t, home, testCase.goos, testCase.variables).SwiftPMRegistries(); got != testCase.want {
			t.Errorf("%s %v: got %s, want %s", testCase.goos, testCase.variables, got, testCase.want)
		}
	}
	touch(t, library)
	if got := machine(t, home, "darwin", map[string]string{"XDG_CONFIG_HOME": xdg}).SwiftPMRegistries(); got != library {
		t.Errorf("darwin with the idiomatic file: got %s", got)
	}
	if got := machine(t, home, "linux", nil).SwiftPMRegistries(); got != dot {
		t.Errorf("linux ignores ~/Library: got %s", got)
	}
	if m := machine(t, "", "linux", nil); m.CocoaPodsRepositories() != "" || m.SwiftPMRegistries() != "" {
		t.Error("no home, no paths")
	}
}

// Conan 2's home is CONAN_HOME (~ the user's home; a relative one is refused),
// else .conan2 in the user's home.
//
// Verifies: REQ-SUP-064
func TestConanHome(t *testing.T) {
	home, directory := t.TempDir(), t.TempDir()
	for _, testCase := range []struct {
		goos      string
		variables map[string]string
		want      string
	}{
		{"linux", nil, filepath.Join(home, ".conan2")},
		{"windows", nil, filepath.Join(home, ".conan2")},
		{"linux", map[string]string{"CONAN_HOME": directory}, directory},
		{"linux", map[string]string{"CONAN_HOME": "~/conan-work"}, filepath.Join(home, "conan-work")},
		{"windows", map[string]string{"CONAN_HOME": `~\conan-work`}, filepath.Join(home, "conan-work")},
		{"windows", map[string]string{"CONAN_HOME": `D:\conan`}, `D:\conan`},
		{"linux", map[string]string{"CONAN_HOME": "relative/home"}, ""},
	} {
		if got := machine(t, home, testCase.goos, testCase.variables).ConanHome(); got != testCase.want {
			t.Errorf("%s %v: got %s, want %s", testCase.goos, testCase.variables, got, testCase.want)
		}
	}
	if got := machine(t, "", "linux", nil).ConanHome(); got != "" {
		t.Errorf("no home: %s", got)
	}
}

// cabal-install reads one configuration file: CABAL_CONFIG, else CABAL_DIR's, else
// ~/.cabal/config while ~/.cabal exists and the XDG one does not, else the XDG one;
// %APPDATA%\cabal\config on Windows.
//
// Verifies: REQ-SUP-064
func TestCabalConfig(t *testing.T) {
	home, xdg, appData := t.TempDir(), t.TempDir(), t.TempDir()
	if got, want := machine(t, home, "linux", nil).CabalConfig(), filepath.Join(home, ".config", "cabal", "config"); got != want {
		t.Errorf("default: got %s, want %s", got, want)
	}
	for _, testCase := range []struct {
		goos      string
		variables map[string]string
		want      string
	}{
		{"linux", map[string]string{"CABAL_CONFIG": "/etc/cabal.config", "CABAL_DIR": "/opt/cabal"}, "/etc/cabal.config"},
		{"linux", map[string]string{"CABAL_DIR": "/opt/cabal"}, filepath.Join("/opt/cabal", "config")},
		{"windows", map[string]string{"APPDATA": appData}, filepath.Join(appData, "cabal", "config")},
		{"linux", map[string]string{"XDG_CONFIG_HOME": xdg}, filepath.Join(xdg, "cabal", "config")},
	} {
		if got := machine(t, home, testCase.goos, testCase.variables).CabalConfig(); got != testCase.want {
			t.Errorf("%s %v: got %s, want %s", testCase.goos, testCase.variables, got, testCase.want)
		}
	}
	// ~/.cabal from an older installation wins until the XDG file exists.
	touch(t, filepath.Join(home, ".cabal", "config"))
	if got, want := machine(t, home, "linux", nil).CabalConfig(), filepath.Join(home, ".cabal", "config"); got != want {
		t.Errorf("legacy: got %s, want %s", got, want)
	}
	touch(t, filepath.Join(home, ".config", "cabal", "config"))
	if got, want := machine(t, home, "linux", nil).CabalConfig(), filepath.Join(home, ".config", "cabal", "config"); got != want {
		t.Errorf("both: got %s, want %s", got, want)
	}
	if got := machine(t, "", "linux", nil).CabalConfig(); got != "" {
		t.Errorf("no home: %s", got)
	}
}

// LuaRocks reads, per Lua version, the file LUAROCKS_CONFIG_5_x or
// LUAROCKS_CONFIG names when it exists, else the XDG one, else ~/.luarocks's;
// %APPDATA%\luarocks's on Windows.
//
// Verifies: REQ-SUP-064
func TestLuaRocksConfigs(t *testing.T) {
	home, appData := t.TempDir(), t.TempDir()
	named := filepath.Join(t.TempDir(), "luarocks.lua")
	touch(t, named)
	touch(t, filepath.Join(home, ".luarocks", "config-5.1.lua"))
	touch(t, filepath.Join(home, ".luarocks", "config-5.4.lua"))
	touch(t, filepath.Join(home, ".config", "luarocks", "config-5.4.lua"))
	touch(t, filepath.Join(appData, "luarocks", "config-5.3.lua"))
	for _, testCase := range []struct {
		goos      string
		variables map[string]string
		want      []string
	}{
		{"linux", nil, []string{filepath.Join(home, ".luarocks", "config-5.1.lua"), filepath.Join(home, ".config", "luarocks", "config-5.4.lua")}},
		{"linux", map[string]string{"LUAROCKS_CONFIG": named}, []string{named}},
		{"linux", map[string]string{"LUAROCKS_CONFIG_5_4": named, "LUAROCKS_CONFIG": filepath.Join(home, "missing.lua")},
			[]string{filepath.Join(home, ".luarocks", "config-5.1.lua"), named}},
		{"windows", map[string]string{"APPDATA": appData}, []string{filepath.Join(appData, "luarocks", "config-5.3.lua")}},
	} {
		if got := machine(t, home, testCase.goos, testCase.variables).LuaRocksConfigs(); !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("%s %v: got %v, want %v", testCase.goos, testCase.variables, got, testCase.want)
		}
	}
}
