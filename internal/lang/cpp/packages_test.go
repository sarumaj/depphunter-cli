package cpp

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// testdata/pkgs: three projects side by side. vcpkg/ has a vcpkg.json with a
// builtin-baseline, "version>=" minimums and overrides, and a tools/ directory
// with its own manifest, no baseline and a registry's; conan-txt/ a
// conanfile.txt with a Conan 2 conan.lock; conan-py/ a conanfile.py with a Conan
// 1 (graph_lock) conan.lock. shared/ has no manifest of its own.

func vcpkg(name, version, requested string, pinned bool) lang.Target {
	return lang.Target{Ecosystem: ecosystemVcpkg, Package: name, Version: version, Requested: requested, Pinned: pinned}
}

func conan(name, version, requested string) lang.Target {
	return lang.Target{Ecosystem: ecosystemConan, Package: name, Version: version, Requested: requested, Pinned: true}
}

// Verifies: REQ-CPP-009, REQ-CPP-012
func TestVcpkg(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/pkgs")
	// cSpell: disable
	langtest.CheckImports(t, results["vcpkg/src/main.cpp"], map[string]lang.Target{
		// An override pins; the minimum it replaced is what was requested.
		`#include <fmt/format.h>`:  vcpkg("fmt", "10.1.1", "", true),
		`#include <openssl/ssl.h>`: vcpkg("openssl", "3.2.0", ">=3.0.8", true),
		// A minimum alone floats. A Boost header belongs to the port of its
		// directory, and to no port the manifest lacks.
		`#include <boost/asio.hpp>`:       vcpkg("boost-asio", ">=1.83.0", "", false),
		`#include <boost/filesystem.hpp>`: external("boost"),
		// No version, but the baseline fixes one: neither pinned nor floating.
		`#include <nlohmann/json.hpp>`: vcpkg("nlohmann-json", "", "", false),
		`#include <zlib.h>`:            vcpkg("zlib", "", "", false),
		`#include "gtest/gtest.h"`:     vcpkg("gtest", "", "", false),
		`#include <Eigen/Dense>`:       vcpkg("eigen3", "", "", false),
		`#include <SDL2/SDL.h>`:        vcpkg("sdl2", "", "", false),
		`#include <GLFW/glfw3.h>`:      vcpkg("glfw3", "", "", false),
		`#include <QtCore/QString>`:    vcpkg("qtbase", "", "", false),
		`#include <curl/curl.h>`:       vcpkg("curl", "", "", false),
		// A feature's dependency.
		`#include <opencv2/core.hpp>`: vcpkg("opencv4", "", "", false),
		// Declared nowhere.
		`#include <spdlog/spdlog.h>`: external("spdlog"),
		`#include <vector>`:          std(ecosystemCppStd, "vector"),
	})
	// tools/ has its own manifest; what it lacks comes from the one above.
	langtest.CheckImports(t, results["vcpkg/tools/check.cpp"], map[string]lang.Target{
		// No version and no baseline: whatever the vcpkg checkout has.
		`#include <catch2/catch_test_macros.hpp>`: {Ecosystem: ecosystemVcpkg, Package: "catch2", Floating: true},
		// The registry claiming acme-* has a baseline.
		`#include <acme-net/client.h>`: vcpkg("acme-net", "", "", false),
		`#include <fmt/core.h>`:        vcpkg("fmt", "10.1.1", "", true),
	})
	// cSpell: enable
}

// Verifies: REQ-CPP-010, REQ-CPP-011, REQ-CPP-012
func TestConan(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/pkgs")
	// cSpell: disable
	langtest.CheckImports(t, results["conan-txt/src/app.cpp"], map[string]lang.Target{
		`#include <zlib.h>`: conan("zlib", "1.2.13", ""),
		// The Conan 2 lock resolves the range.
		`#include <spdlog/spdlog.h>`: conan("spdlog", "1.12.0", "[>=1.11 <2]"),
		// Only the lock has fmt, which spdlog needs: installed, so declared.
		`#include <fmt/format.h>`:      conan("fmt", "10.1.1", ""),
		`#include <nlohmann/json.hpp>`: conan("nlohmann_json", "3.11.2", ""),
		// Commented out in conanfile.txt.
		`#include <boost/asio.hpp>`: external("boost"),
	})
	langtest.CheckImports(t, results["conan-py/main.cpp"], map[string]lang.Target{
		`#include <openssl/evp.h>`:    conan("openssl", "1.1.1t", ""),
		`#include <zlib.h>`:           conan("zlib", "1.2.13", "[~1.2]"),
		`#include <curl/curl.h>`:      conan("libcurl", "8.4.0", ""),
		`#include <catch2/catch.hpp>`: conan("catch2", "3.4.0", ""),
		// An f-string and a commented-out requirement are not read.
		`#include <boost/any.hpp>`:                external("boost"),
		`#include <Poco/Net/HTTPClientSession.h>`: external("Poco"),
	})
	// cSpell: enable
}

// A file under no manifest takes a library from the project's other manifests,
// the shallowest first and then by directory name; a file under one does not
// (TestVcpkg: spdlog, which only conan-txt/ declares, stays external there).
//
// Verifies: REQ-CPP-012
func TestManifestElsewhere(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/pkgs")
	langtest.CheckImports(t, results["shared/log.cpp"], map[string]lang.Target{
		`#include <spdlog/spdlog.h>`:   conan("spdlog", "1.12.0", "[>=1.11 <2]"),
		`#include <fmt/format.h>`:      conan("fmt", "10.1.1", ""), // conan-txt before vcpkg
		`#include <acme-net/client.h>`: vcpkg("acme-net", "", "", false),
	})
}

// Verifies: REQ-CPP-010
func TestConanRequirementsWithoutLock(t *testing.T) {
	names := func(packages []*declaredPackage) map[string]lang.Target {
		out := map[string]lang.Target{}
		for _, p := range packages {
			out[p.name] = p.target()
		}
		return out
	}
	floating := lang.Target{Ecosystem: ecosystemConan, Package: "zlib", Version: "[~1.2]"}
	// cSpell: disable
	got := names(readConanfilePy([]byte(`
class App(ConanFile):
    requires = "openssl/1.1.1t", "zlib/[~1.2]"
    tool_requires = [
        "cmake/3.27.1",  # "ninja/1.11.1" is not required
    ]
    def requirements(self):
        self.requires("libcurl/8.4.0@user/stable#rrev")
        self.requires(f"boost/{self.version}")
        # self.requires("poco/1.12.4")
`)))
	want := map[string]lang.Target{
		"openssl": conan("openssl", "1.1.1t", ""), "zlib": floating,
		"cmake": conan("cmake", "3.27.1", ""), "libcurl": conan("libcurl", "8.4.0", ""),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("conanfile.py: got %v, want %v", got, want)
	}
	got = names(readConanfileTxt([]byte("[requires]\nzlib/[~1.2]\n[generators]\nCMakeDeps\n[tool_requires]\ncmake/3.27.1\n")))
	want = map[string]lang.Target{"zlib": floating, "cmake": conan("cmake", "3.27.1", "")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("conanfile.txt: got %v, want %v", got, want)
	}
	// cSpell: enable
}

// A Conan 1 lock gives the graph, so --resolve-depth walks it; a Conan 2 lock is
// a flat list and gives no edges.
//
// Verifies: REQ-CPP-011
func TestConanLockGraph(t *testing.T) {
	r := newResolver(t.TempDir(), langtest.Files(t, "testdata/pkgs"))
	for packageTarget, want := range map[lang.Target][]lang.Target{
		conan("libcurl", "8.4.0", ""):    {conan("openssl", "1.1.1t", ""), conan("zlib", "1.2.13", "")},
		conan("openssl", "1.1.1t", ""):   {conan("zlib", "1.2.13", "")},
		conan("zlib", "1.2.13", ""):      nil,
		conan("spdlog", "1.12.0", ""):    nil,
		vcpkg("fmt", "10.1.1", "", true): nil,
	} {
		if got := r.Dependencies(packageTarget); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", packageTarget.Package, got, want)
		}
	}
}

// Verifies: REQ-CPP-012
func TestCandidates(t *testing.T) {
	// cSpell: disable
	for include, want := range map[string][]string{
		"boost/asio/io_context.hpp": {"boost-asio", "boost", "libboost"},
		"boost/lexical_cast.hpp":    {"boost-lexical-cast", "boost", "libboost"},
		"nlohmann/json.hpp":         {"nlohmann", "nlohmann-json", "libnlohmann"},
		"uv.h":                      {"uv", "libuv"},
		"libxml/parser.h":           {"libxml", "libxml2"},
		"QtWidgets/QWidget":         {"qtwidgets", "libqtwidgets", "qtbase", "qt5-base", "qt6", "qt5", "qt"},
	} {
		if got := candidates(include); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", include, got, want)
		}
	}
	// cSpell: enable
}
