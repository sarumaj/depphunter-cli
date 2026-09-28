package cmake

import "strings"

// stdModules are the modules CMake ships (include(FetchContent)), lower-cased to the
// spelling CMake has. Find modules are found by their "Find" prefix, not listed.
//
// Implements: REQ-CMAKE-005
var stdModules = names(`AndroidTestUtilities BundleUtilities CheckCCompilerFlag
CheckCSourceCompiles CheckCSourceRuns CheckCXXCompilerFlag CheckCXXSourceCompiles
CheckCXXSourceRuns CheckCXXSymbolExists CheckCompilerFlag CheckFortranCompilerFlag
CheckFortranFunctionExists CheckFortranSourceCompiles CheckFortranSourceRuns
CheckFunctionExists CheckIPOSupported CheckIncludeFile CheckIncludeFileCXX
CheckIncludeFiles CheckLanguage CheckLibraryExists CheckLinkerFlag
CheckOBJCCompilerFlag CheckOBJCSourceCompiles CheckOBJCSourceRuns
CheckOBJCXXCompilerFlag CheckOBJCXXSourceCompiles CheckOBJCXXSourceRuns
CheckPIESupported CheckPrototypeDefinition CheckSourceCompiles CheckSourceRuns
CheckStructHasMember CheckSymbolExists CheckTypeSize CheckVariableExists
CMakeAddFortranSubdirectory CMakeBackwardCompatibilityCXX CMakeDependentOption
CMakeDetermineVSServicePack CMakeExpandImportedTargets CMakeFindDependencyMacro
CMakeFindFrameworks CMakeFindPackageMode CMakeForceCompiler CMakeGraphVizOptions
CMakePackageConfigHelpers CMakeParseArguments CMakePrintHelpers
CMakePrintSystemInformation CMakePushCheckState CMakeVerifyManifest CPack
CPackComponent CPackIFW CPackIFWConfigureFile CSharpUtilities CTest
CTestCoverageCollectGCOV CTestScriptMode CTestUseLaunchers Dart DeployQt4
Documentation ExternalData ExternalProject FeatureSummary FetchContent
FindPackageHandleStandardArgs FindPackageMessage FortranCInterface
GenerateExportHeader GetPrerequisites GNUInstallDirs GoogleTest
InstallRequiredSystemLibraries MacroAddFileDependencies ProcessorCount
SelectLibraryConfigurations SquishTestScript TestBigEndian TestCXXAcceptsFlag
TestForANSIForScope TestForANSIStreamHeaders TestForSSTREAM TestForSTDNamespace
UseEcos UseJava UseJavaClassFilelist UseJavaSymlinks UseSWIG UsewxWidgets Use_wxWindows
WriteBasicConfigVersionFile WriteCompilerDetectionHeader AddFileDependencies
CMakeDetermineCompilerId CMakeFindJavaCommon`)

// systemFinds are the packages find_package reads with a find module CMake ships
// for a tool, a language runtime or the platform rather than for a library a package
// manager would install: they are CMake's (the cmake-std island, as Find<Name>),
// like its other modules.
//
// Implements: REQ-CMAKE-006
var systemFinds = names(`Threads OpenMP PkgConfig Python Python2 Python3 PythonInterp
PythonLibs Java JNI Doxygen Git CUDA CUDAToolkit MPI OpenGL OpenCL Vulkan X11 Perl
PerlLibs BISON FLEX SWIG Matlab Ruby Subversion Hg LATEX Wget Patch Backtrace Intl
Iconv Gettext UnixCommands Cygwin Msys HIP Tclsh TCL TclStub Squish CxxTest
EnvModules`)

// findIncludes are the headers of the libraries whose find_package name is not their
// include directory's, lower-cased: find_package(ZLIB) is <zlib.h>. A library is
// attributed as its header would be (cpp.Packages.Library), so find_package(ZLIB)
// and #include <zlib.h> are one node. Anything else is taken to be included as
// <name/...> in lower case (find_package(fmt), <fmt/core.h>).
//
// Implements: REQ-CMAKE-006
var findIncludes = map[string]string{
	// cSpell: disable
	"zlib": "zlib.h", "bzip2": "bzlib.h", "png": "png.h", "jpeg": "jpeglib.h",
	"libjpeg-turbo": "turbojpeg.h", "tiff": "tiffio.h", "gif": "gif_lib.h",
	"sqlite3": "sqlite3.h", "sqlite": "sqlite3.h", "expat": "expat.h",
	"libxml2": "libxml/", "libxslt": "libxslt/", "freetype": "ft2build.h",
	"gtest": "gtest/", "googletest": "gtest/", "gmock": "gmock/",
	"nlohmann_json": "nlohmann/json.hpp", "eigen3": "Eigen/", "eigen": "Eigen/",
	"protobuf": "google/protobuf/", "grpc": "grpcpp/", "absl": "absl/",
	"glfw3": "GLFW/", "glfw": "GLFW/", "glew": "GL/glew.h", "sdl2": "SDL2/", "sdl3": "SDL3/",
	"opencv": "opencv2/", "tbb": "tbb/", "lz4": "lz4.h", "zstd": "zstd.h",
	"liblzma": "lzma.h", "lzma": "lzma.h", "libuv": "uv.h", "lua": "lua.h",
	"luajit": "luajit.h", "libevent": "event2/", "zeromq": "zmq.h", "cppzmq": "zmq.hpp",
	"libarchive": "archive.h", "hdf5": "hdf5.h", "cxxopts": "cxxopts.hpp",
	"libssh2": "libssh2.h", "libssh": "libssh/", "libgit2": "git2.h", "yajl": "yajl/",
	"pugixml": "pugixml.hpp", "tinyxml2": "tinyxml2.h", "rapidjson": "rapidjson/",
	"libzip": "zip.h", "minizip": "minizip/", "utf8cpp": "utf8.h", "glm": "glm/",
	"assimp": "assimp/", "bullet": "btBulletDynamicsCommon.h", "ogg": "ogg/",
	"vorbis": "vorbis/", "flac": "FLAC/", "openal": "AL/", "libusb": "libusb.h",
	"pcre2": "pcre2.h", "pcre": "pcre.h", "unofficial-sqlite3": "sqlite3.h",
	"libpqxx": "pqxx/", "postgresql": "libpq-fe.h", "mysql": "mysql.h", "mariadb": "mysql.h",
	"xercesc": "xercesc/", "blend2d": "blend2d.h", "wxwidgets": "wx/", "gtk3": "gtk/",
	"catch2": "catch2/", "doctest": "doctest/", "benchmark": "benchmark/",
	"ms-gsl": "gsl/", "microsoft.gsl": "gsl/", "range-v3": "range/", "fmt": "fmt/",
	"spdlog": "spdlog/", "cli11": "CLI/", "magic_enum": "magic_enum.hpp",
	"yaml-cpp": "yaml-cpp/", "jsoncpp": "json/", "cereal": "cereal/", "msgpack": "msgpack.hpp",
	"flatbuffers": "flatbuffers/", "capnproto": "capnp/", "thrift": "thrift/",
	"llvm": "llvm/", "clang": "clang/", "mlir": "mlir/",
	// cSpell: enable
}

// qtLike are the frameworks find_package names by major version whose components
// are include directories of their own: find_package(Qt6 COMPONENTS Core) is
// <QtCore/...>.
func qtLike(name string) bool {
	l := strings.ToLower(name)
	return l == "qt" || l == "qt5" || l == "qt6" || l == "qt4"
}

// representative is the include a find_package name (and component, for Boost and Qt)
// is attributed as, and the names a package manager may give the library beyond the
// include's (vcpkg's "unofficial-sqlite3" is port sqlite3).
func representative(name, component string) (include string, extra []string) {
	lower := strings.ToLower(name)
	switch {
	case lower == "boost" && component != "":
		return "boost/" + strings.ToLower(component), nil
	case lower == "boost":
		return "boost/", nil
	case qtLike(name) && component != "":
		return "Qt" + component + "/", nil
	case qtLike(name):
		return "QtCore/", nil
	case strings.HasPrefix(lower, "qt5") || strings.HasPrefix(lower, "qt6"):
		return "Qt" + name[3:] + "/", nil // find_package(Qt5Widgets)
	}
	bare := strings.TrimPrefix(lower, "unofficial-")
	if include, ok := findIncludes[lower]; ok {
		return include, []string{bare, name}
	}
	if include, ok := findIncludes[bare]; ok {
		return include, []string{bare}
	}
	return bare + "/", []string{name}
}

// branches are the ref names FetchContent and CPM are commonly pointed at that move
// with every commit.
var branches = names(`main master develop development dev trunk HEAD latest stable
nightly`)

// names makes a lookup set of words, keyed in lower case, holding the spelling given.
func names(words string) map[string]string {
	m := map[string]string{}
	for _, w := range strings.Fields(words) {
		m[strings.ToLower(w)] = w
	}
	return m
}
