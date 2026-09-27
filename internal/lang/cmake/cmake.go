// Package cmake analyzes CMake builds: CMakeLists.txt, *.cmake and *.cmake.in
// files, and CMakePresets.json / CMakeUserPresets.json.
//
// A build file depends on the directories it adds (add_subdirectory: the
// directory's CMakeLists.txt), the files and modules it includes (a module on
// CMAKE_MODULE_PATH is the project's file; one CMake ships is the hidden cmake-std
// island), the source files of its targets and configure_file()'s templates - all
// project files - and on the libraries it finds and fetches. find_package(X) is the
// library an #include of X's headers is attributed to by the cpp plugin
// (cpp.Packages.Library): a vcpkg or Conan package when a manifest declares it,
// else the c-external library named like X's include directory, so the build file
// and the sources including <X/...> meet on one node. Content fetched with
// FetchContent, ExternalProject or CPM.cmake is the cmake-fetch island, named by its
// repository or download URL; pkg_check_modules() modules the pkg-config island.
//
// Paths are evaluated from literals, the variables the file sets (resolved against
// the directories above it in the resolver) and CMake's own variables for the
// source directories (resolve.go).
//
// Files are read by a scanner of the plugin's own (lex.go), not the vendored
// tree-sitter cmake grammar, which was accurate but some 100 times slower
// (REQ-CMAKE-010).
package cmake

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoFetch = "cmake-fetch" // FetchContent, ExternalProject and CPM.cmake content
	ecoPkg   = "pkg-config"  // pkg_check_modules() modules
	ecoStd   = "cmake-std"   // the modules CMake ships
)

// Implements: REQ-CMAKE-001
type Plugin struct{}

func (Plugin) Name() string { return "cmake" }
func (Plugin) Version() int { return 1 }

// presets reports whether a file is a CMake presets file.
func presets(p string) bool {
	base := path.Base(p)
	return base == "CMakePresets.json" || base == "CMakeUserPresets.json"
}

// Claims takes CMakeLists.txt, *.cmake, *.cmake.in (package configuration
// templates) and the presets files.
//
// Implements: REQ-CMAKE-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	base := path.Base(f.Path)
	lower := strings.ToLower(base)
	return base == "CMakeLists.txt" || strings.HasSuffix(lower, ".cmake") ||
		strings.HasSuffix(lower, ".cmake.in") || presets(f.Path)
}

// Implements: REQ-CMAKE-005, REQ-CMAKE-006, REQ-CMAKE-007, REQ-CMAKE-011
func (Plugin) Ecosystems() []lang.Ecosystem {
	return append(cpp.PackageEcosystems(),
		lang.Ecosystem{ID: ecoFetch, Name: "CMake fetched content"},
		lang.Ecosystem{ID: ecoPkg, Name: "pkg-config modules"},
		lang.Ecosystem{ID: ecoStd, Name: "CMake modules", Std: true},
	)
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Extract depends on the file's name only through its extension: of the claimed
// files, only the presets files are .json, and every other one is CMake language.
//
// Implements: REQ-CMAKE-002, REQ-CMAKE-008, REQ-CMAKE-009
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	if strings.EqualFold(path.Ext(f.Path), ".json") {
		return readPresets(src), nil
	}
	info := analyze(lex(src))
	return &lang.Extraction{Imports: info.imports, Symbols: info.symbols.List()}, nil
}
