package cmake

import (
	"reflect"
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is a small C++ project built with CMake: a root CMakeLists.txt putting
// cmake/ on CMAKE_MODULE_PATH, including CMake's modules, its own modules by name
// and by path, finding libraries (some declared in vcpkg.json, Boost and Qt
// components, a system package, one the project fetches) and adding
// subdirectories, one through a variable; cmake/Deps.cmake fetching content with
// FetchContent (a commit, a tag, a branch, no tag, a hashed release asset, a plain
// download), ExternalProject (a tarball in the repository) and CPM (shorthand and
// keywords) and a function whose parameters cannot be known; src/ taking its
// sources from a list variable, a variable of the directory above, the
// directories CMake names (CMAKE_CURRENT_SOURCE_DIR, PROJECT_SOURCE_DIR,
// <Project>_SOURCE_DIR) and a generator expression; tests/ as a project of its
// own; tools/ computing the root with get_filename_component; a package
// configuration template; and a presets file.
//
// Verifies: REQ-CMAKE-001, REQ-CMAKE-002, REQ-CMAKE-003, REQ-CMAKE-004, REQ-CMAKE-005
// Verifies: REQ-CMAKE-006, REQ-CMAKE-007, REQ-CMAKE-008, REQ-CMAKE-009, REQ-CMAKE-010
// Verifies: REQ-CMAKE-011
func TestBuildFilesAndPackages(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	local := func(p string) lang.Target { return lang.Target{Local: p} }
	std := func(name string) lang.Target { return lang.Target{Ecosystem: ecoStd, Package: name} }
	external := func(name string) lang.Target {
		return lang.Target{Ecosystem: "c-external", Package: name, Unresolved: true}
	}
	vcpkg := func(name string) lang.Target { return lang.Target{Ecosystem: "vcpkg", Package: name, Floating: true} }
	fetch := func(name, version string, pinned, floating bool) lang.Target {
		return lang.Target{Ecosystem: ecoFetch, Package: name, Version: version, Pinned: pinned, Floating: floating}
	}
	gtest := fetch("github.com/google/googletest", "f8d7d77c06936315286eb55f8de22cd23c188571", true, false)
	catch2 := fetch("github.com/catchorg/Catch2", "v3.5.2", false, false)
	json := fetch("github.com/nlohmann/json", "v3.11.3", true, false)
	magicEnum := fetch("github.com/Neargye/magic_enum", "v0.9.5", false, false)
	imports := map[string]map[string]lang.Target{
		"CMakeLists.txt": {
			"include(FetchContent)":                  std("FetchContent"),
			"include(GNUInstallDirs)":                std("GNUInstallDirs"),
			"include(Warnings)":                      local("cmake/Warnings.cmake"),
			"include(cmake/Deps.cmake)":              local("cmake/Deps.cmake"),
			"include(NotAnywhere)":                   {},
			"find_package(Threads)":                  std("FindThreads"),
			"find_package(fmt)":                      vcpkg("fmt"),
			"find_package(ZLIB)":                     vcpkg("zlib"),
			"find_package(Boost filesystem)":         vcpkg("boost-filesystem"),
			"find_package(Boost system)":             external("boost"),
			"find_package(OpenSSL)":                  external("openssl"),
			"find_package(Qt6 Core)":                 external("QtCore"),
			"find_package(Qt6 Widgets)":              external("QtWidgets"),
			"find_package(nlohmann_json)":            json, // fetched by cmake/Deps.cmake
			"find_package(Eigen3)":                   external("Eigen"),
			"find_package(Catch2)":                   catch2,
			"find_package(PkgConfig)":                std("FindPkgConfig"),
			"pkg_check_modules(GLIB glib-2.0>=2.40)": {Ecosystem: ecoPkg, Package: "glib-2.0", Version: ">=2.40"},
			"pkg_check_modules(GLIB gio-2.0)":        {Ecosystem: ecoPkg, Package: "gio-2.0"},
			"add_subdirectory(src)":                  local("src/CMakeLists.txt"),
			"add_subdirectory(${SHOP_TOOLS})":        local("tools/CMakeLists.txt"),
			"add_subdirectory(tests)":                local("tests/CMakeLists.txt"),
			"add_subdirectory(missing)":              {},
			"configure_file(config.h.in)":            local("config.h.in"),
		},
		"cmake/Deps.cmake": {
			"FetchContent_Declare(googletest)":             gtest,
			"FetchContent_Declare(Catch2)":                 catch2,
			"FetchContent_Declare(json)":                   json,
			"FetchContent_Declare(spdlog)":                 fetch("github.com/gabime/spdlog", "origin/main", false, true),
			"FetchContent_Declare(cli11)":                  fetch("github.com/CLIUtils/CLI11", "", false, true),
			"FetchContent_Declare(zip)":                    fetch("example.com/downloads/zip-1.0.tar.gz", "", false, true),
			"ExternalProject_Add(legacy)":                  local("third_party/legacy-1.2.tar.gz"),
			"include(CPM)":                                 local("cmake/CPM.cmake"),
			"CPMAddPackage(gh:TartanLlama/expected@1.1.0)": fetch("github.com/TartanLlama/expected", "v1.1.0", false, false),
			"CPMAddPackage(magic_enum)":                    magicEnum,
			"CPMAddPackage(doctest)":                       fetch("github.com/doctest/doctest", "ae7a13539fb71f270b87eb2e874fbac80bc8dda2", true, false),
			"FetchContent_MakeAvailable(googletest)":       gtest,
			"FetchContent_MakeAvailable(Catch2)":           catch2,
			"FetchContent_MakeAvailable(json)":             json,
			"FetchContent_MakeAvailable(unknown)":          {},
		},
		"cmake/ShopConfig.cmake.in": {
			"include(CMakeFindDependencyMacro)":                    std("CMakeFindDependencyMacro"),
			"find_dependency(fmt)":                                 vcpkg("fmt"),
			"find_dependency(Threads)":                             std("FindThreads"),
			"include(${CMAKE_CURRENT_LIST_DIR}/ShopTargets.cmake)": {},
		},
		"cmake/Warnings.cmake":  {},
		"cmake/CPM.cmake":       {},
		"cmake/toolchain.cmake": {},
		"src/CMakeLists.txt": {
			"add_library(shop_core core.cpp)":                              local("src/core.cpp"),
			"add_library(shop_core util.cpp)":                              local("src/util.cpp"),
			"add_library(shop_core ${SHOP_INCLUDE}/shop/core.hpp)":         local("include/shop/core.hpp"),
			"add_executable(shop main.cpp)":                                local("src/main.cpp"),
			"add_executable(shop ${CMAKE_CURRENT_SOURCE_DIR}/util.cpp)":    local("src/util.cpp"),
			"add_executable(shop generated.cpp)":                           {},
			"target_sources(shop ${Shop_SOURCE_DIR}/include/shop/api.hpp)": local("include/shop/api.hpp"),
			"include(Warnings)":                                            local("cmake/Warnings.cmake"),
			"include(${Shop_SOURCE_DIR}/cmake/Warnings.cmake)":             local("cmake/Warnings.cmake"),
			"CPMGetPackage(magic_enum)":                                    magicEnum,
			"add_custom_target(docs ../README.md)":                         local("README.md"),
		},
		"tests/CMakeLists.txt": {
			"find_package(Shop)":                       local("CMakeLists.txt"),
			"find_package(GTest)":                      gtest,
			"include(GoogleTest)":                      std("GoogleTest"),
			"include(CTest)":                           std("CTest"),
			"add_executable(shop_tests test_main.cpp)": local("tests/test_main.cpp"),
			"add_executable(shop_tests ${PROJECT_SOURCE_DIR}/test_cart.cpp)": local("tests/test_cart.cpp"),
			"add_executable(shop_tests ${CMAKE_SOURCE_DIR}/src/util.cpp)":    local("src/util.cpp"),
		},
		"tools/CMakeLists.txt": {
			"add_executable(shopctl ctl.cpp)":              local("tools/ctl.cpp"),
			"add_executable(shopctl ${ROOT}/src/util.cpp)": local("src/util.cpp"),
		},
		"CMakePresets.json": {
			`"include": "presets/base.json"`:                        local("presets/base.json"),
			`"toolchainFile": "${sourceDir}/cmake/toolchain.cmake"`: local("cmake/toolchain.cmake"),
		},
	}
	for file, want := range imports {
		t.Run(file, func(t *testing.T) { langtest.CheckImports(t, res[file], want) })
	}
	if len(res) != len(imports) {
		t.Errorf("analyzed %d files, want %d", len(res), len(imports))
	}
	symbols := map[string]map[string]string{
		"CMakeLists.txt":       {"Shop": "project", "SHOP_TESTS": "option", "SHOP_LOG_LEVEL": "cache"},
		"cmake/Deps.cmake":     {"shop_fetch": "function"},
		"cmake/Warnings.cmake": {"shop_warnings": "function", "shop_option": "macro"},
		"src/CMakeLists.txt": {
			"shop_core":  "library",
			"Shop::core": "alias",
			"shop":       "executable",
			"docs":       "target",
		},
		"tests/CMakeLists.txt": {"shop_tests": "project", "shop_tests@6": "executable"},
		"tools/CMakeLists.txt": {"shopctl": "executable"},
		"CMakePresets.json":    {"default": "preset", "vcpkg": "preset", "default@16": "preset"},
	}
	for file, want := range symbols {
		if got := langtest.Symbols(t, res[file]); !reflect.DeepEqual(got, want) {
			t.Errorf("%s symbols: got %v, want %v", file, got, want)
		}
	}
}

// A library found by the build and included by the sources is one node: the cpp
// plugin's attribution of src/deps.cpp's includes equals the cmake plugin's of the
// matching find_package calls, declared in vcpkg.json, fetched or neither. Headers of
// fetched content are attributed to it by its declared name (doctest, magic_enum)
// or its repository's (googletest's gtest by the alias, nlohmann/json); without the
// cmake plugin's reader they stay c-external.
//
// Verifies: REQ-CMAKE-006, REQ-CPP-017
func TestFindPackageMeetsIncludes(t *testing.T) {
	build := langtest.Imports(t, langtest.Analyze(t, Plugin{}, "testdata/repo")["CMakeLists.txt"])
	build2 := langtest.Imports(t, langtest.Analyze(t, Plugin{}, "testdata/repo")["tests/CMakeLists.txt"])
	deps := langtest.Imports(t, langtest.Analyze(t, Plugin{}, "testdata/repo")["cmake/Deps.cmake"])
	sources := langtest.Imports(t, langtest.Analyze(t, cpp.Plugin{Fetches: Plugin{}}, "testdata/repo")["src/deps.cpp"])
	for include, find := range map[string]lang.Target{
		"#include <nlohmann/json.hpp>":            build["find_package(nlohmann_json)"],
		"#include <boost/system/error_code.hpp>":  build["find_package(Boost system)"],
		"#include <QtWidgets/QWidget>":            build["find_package(Qt6 Widgets)"],
		"#include <zlib.h>":                       build["find_package(ZLIB)"],
		"#include <gtest/gtest.h>":                build2["find_package(GTest)"],
		"#include <doctest/doctest.h>":            deps["CPMAddPackage(doctest)"],
		"#include <magic_enum.hpp>":               deps["CPMAddPackage(magic_enum)"],
		"#include <catch2/catch_test_macros.hpp>": deps["FetchContent_Declare(Catch2)"],
		"#include <cxxopts.hpp>":                  {Ecosystem: "c-external", Package: "cxxopts", Unresolved: true},
	} {
		if got, ok := sources[include]; !ok {
			t.Errorf("%s: not captured", include)
		} else if got != find || find == (lang.Target{}) {
			t.Errorf("%s: %+v, but the build gives %+v", include, got, find)
		}
	}
	alone := langtest.Imports(t, langtest.Analyze(t, cpp.Plugin{}, "testdata/repo")["src/deps.cpp"])
	if got := alone["#include <doctest/doctest.h>"]; got.Ecosystem != "c-external" {
		t.Errorf("without a reader: %+v", got)
	}
	if !slices.Contains(cpp.Plugin{Fetches: Plugin{}}.Ecosystems(), Plugin{}.FetchIsland()) ||
		slices.Contains(cpp.Plugin{}.Ecosystems(), Plugin{}.FetchIsland()) {
		t.Error("the cpp plugin declares the fetched content's island only with a reader")
	}
	// Content named differently from its repository is found by the repository's
	// name; content in the repository is not fetched.
	root := langtest.Write(t, map[string]string{
		"CMakeLists.txt": "FetchContent_Declare(ext_opts GIT_REPOSITORY https://github.com/jarro2783/cxxopts GIT_TAG v3.2.0)\n" +
			"FetchContent_Declare(vendored URL ${CMAKE_CURRENT_LIST_DIR}/vendored.tar.gz)\n",
		"vendored.tar.gz": "",
		"main.cpp":        "#include <cxxopts.hpp>\n#include <vendored/v.h>\n",
	})
	langtest.CheckImports(t, langtest.Analyze(t, cpp.Plugin{Fetches: Plugin{}}, root)["main.cpp"], map[string]lang.Target{
		"#include <cxxopts.hpp>":  {Ecosystem: ecoFetch, Package: "github.com/jarro2783/cxxopts", Version: "v3.2.0"},
		"#include <vendored/v.h>": {Ecosystem: "c-external", Package: "vendored", Unresolved: true},
	})
	// No CMake file, or one that is garbage, fetches nothing.
	for _, files := range []map[string]string{
		{"main.cpp": "#include <doctest/doctest.h>\n"},
		{"CMakeLists.txt": "\x00FetchContent_Declare(doctest GIT_REPOSITORY\n((", "x.cmake": "CPMAddPackage(NAME)\nFetchContent_Declare()\nFetchContent_Declare(x URL)"},
	} {
		if got := (Plugin{}).Fetched(langtest.Files(t, langtest.Write(t, files))); len(got) != 0 {
			t.Errorf("%v: fetched %+v", files, got)
		}
	}
}

// Verifies: REQ-CMAKE-002
func TestLex(t *testing.T) {
	src := "\xef\xbb\xbfset(A \"x\\\"y\\\n z\" [==[raw ]] ${B}]==] b\\;c)\n" +
		"#[[ include(Hidden)\n]] # add_subdirectory(no)\n" +
		"if((A AND B) OR C) # trailing\n" +
		"Add_SubDirectory ( src ) garbage \"quoted\"\n" +
		"list(APPEND L a;b # comment\n  c)\n"
	got := lex([]byte(src))
	want := []command{
		{name: "set", cased: "set", line: 1, args: []arg{
			{text: "A", line: 1}, {text: "x\"y z", quoted: true, line: 1},
			{text: "raw ]] ${B}", raw: true, line: 2}, {text: `b\;c`, line: 2},
		}},
		{name: "if", cased: "if", line: 5, args: []arg{
			{text: "(", line: 5}, {text: "A", line: 5}, {text: "AND", line: 5}, {text: "B", line: 5},
			{text: ")", line: 5}, {text: "OR", line: 5}, {text: "C", line: 5},
		}},
		{name: "add_subdirectory", cased: "Add_SubDirectory", line: 6, args: []arg{{text: "src", line: 6}}},
		{name: "list", cased: "list", line: 7, args: []arg{
			{text: "APPEND", line: 7}, {text: "L", line: 7}, {text: "a;b", line: 7}, {text: "c", line: 8},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lex:\n got %+v\nwant %+v", got, want)
	}
	// Unterminated constructs end the file rather than looping or panicking.
	for _, s := range []string{`f("abc`, "f([[abc", "#[[abc", `f(a\`, "f(", "f(${", `f("\`} {
		lex([]byte(s))
		analyze(lex([]byte(s)))
	}
}

// Verifies: REQ-CMAKE-003
func TestExpand(t *testing.T) {
	vars := map[string]string{"P": "SHOP", "SHOP_DIR": "src", "L": "a;b"}
	lookup := func(n string) (string, bool) { v, ok := vars[n]; return v, ok }
	for in, want := range map[string]string{
		"${${P}_DIR}/x.cpp": "src/x.cpp",
		"${L}":              "a;b",
		"${Q}/x":            "${Q}/x",
		"${${Q}_DIR}":       "${${Q}_DIR}",
		"$ENV{HOME}/x":      "$ENV{HOME}/x",
		"${unterminated":    "${unterminated",
	} {
		if got, _ := expand(in, lookup, true); got != want {
			t.Errorf("expand(%q) = %q, want %q", in, got, want)
		}
	}
	if _, ok := expand("${Q}/x", lookup, false); ok {
		t.Error("an unknown variable must fail when not kept")
	}
}

// Verifies: REQ-CMAKE-007
func TestCPMShorthand(t *testing.T) {
	for in, want := range map[string][4]string{
		"gh:fmtlib/fmt@10.2.1":                     {"https://github.com/fmtlib/fmt", "", "", "10.2.1"},
		"gl:group/proj#main":                       {"https://gitlab.com/group/proj", "", "main", ""},
		"bb:team/repo@1.0#v1.0-fix":                {"https://bitbucket.org/team/repo", "", "v1.0-fix", "1.0"},
		"https://github.com/a/b.git@2.0.0":         {"https://github.com/a/b.git", "", "", "2.0.0"},
		"https://github.com/a/b.git#abc":           {"https://github.com/a/b.git", "", "abc", ""},
		"https://example.com/x-1.0.zip":            {"", "https://example.com/x-1.0.zip", "", ""},
		"https://user@example.com/files/x-1.0.zip": {"", "https://user@example.com/files/x-1.0.zip", "", ""},
	} {
		repo, url, tag, version := cpmShorthand(in)
		if got := [4]string{repo, url, tag, version}; got != want {
			t.Errorf("cpmShorthand(%q) = %q, want %q", in, got, want)
		}
	}
}

// Verifies: REQ-CMAKE-007
func TestFetchedNaming(t *testing.T) {
	r := newResolver([]*scan.File{{Path: "CMakeLists.txt"}})
	for in, want := range map[string]lang.Target{
		"url|https://github.com/google/googletest/archive/03597a01ee50ed33e9dfd640b249b4be3799d395.zip|": {
			Ecosystem: ecoFetch, Package: "github.com/google/googletest", Version: "03597a01ee50ed33e9dfd640b249b4be3799d395", Pinned: true,
		},
		"url|https://github.com/fmtlib/fmt/archive/refs/tags/10.2.1.tar.gz|": {
			Ecosystem: ecoFetch, Package: "github.com/fmtlib/fmt", Version: "10.2.1",
		},
		"url|https://github.com/a/b/archive/refs/heads/main.zip|": {
			Ecosystem: ecoFetch, Package: "github.com/a/b", Version: "main", Floating: true,
		},
		"url|https://gitlab.com/libeigen/eigen/-/archive/3.4.0/eigen-3.4.0.tar.gz|SHA256=abc": {
			Ecosystem: ecoFetch, Package: "gitlab.com/libeigen/eigen", Version: "eigen-3.4.0", Pinned: true,
		},
		"url|https://example.com/x.tar.gz?raw=1|MD5=0123": {
			Ecosystem: ecoFetch, Package: "example.com/x.tar.gz", Version: "MD5=0123", Pinned: true,
		},
		"git|git@github.com:Org/Repo.git|v1.2.3": {Ecosystem: ecoFetch, Package: "github.com/Org/Repo", Version: "v1.2.3"},
		"git|https://github.com/a/b|master":      {Ecosystem: ecoFetch, Package: "github.com/a/b", Version: "master", Floating: true},
		"git|https://github.com/a/b|${UNKNOWN}":  {Ecosystem: ecoFetch, Package: "github.com/a/b"},
		"git|${UNKNOWN}/b|v1":                    {},
	} {
		kind, rest, _ := cut3(in)
		if got := r.fetched("CMakeLists.txt", lang.RawImport{Name: kind, Module: rest}); got != want {
			t.Errorf("%s: got %+v, want %+v", in, got, want)
		}
	}
}

func cut3(s string) (kind, module string, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '|' {
			rest := s[i+1:]
			for j := len(rest) - 1; j >= 0; j-- {
				if rest[j] == '|' {
					return s[:i], rest[:j] + "\n" + rest[j+1:], true
				}
			}
		}
	}
	return "", "", false
}
