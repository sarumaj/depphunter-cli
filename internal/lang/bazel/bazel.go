// Package bazel analyzes Bazel's Starlark files: BUILD and BUILD.bazel (the
// targets of a package), .bzl extensions, MODULE.bazel (Bzlmod) and the legacy
// WORKSPACE files.
//
// A BUILD file's targets are its symbols (//pkg:name, kind = the rule or macro).
// Their label attributes (srcs, hdrs, deps, data, ...) link the BUILD file to the
// source files it names - glob() patterns expanded against the package's files,
// not crossing into subpackages - and to the BUILD file of each package a label
// names in the same repository. load() links to the .bzl file. A label naming
// another repository resolves to what declares that repository: a bazel_dep (a
// module of the Bazel Central Registry, the bazel island), a WORKSPACE
// http_archive or git_repository (a download named by its URL, the bazel-repo
// island), a local_path_override or local_repository (a directory of the
// project), or a module extension's hub repository - rules_jvm_external's
// @maven, rules_python's pip hubs, Gazelle's go_deps, rules_js' npm and
// rules_rust's crate_universe - which name Maven, PyPI, Go, npm and crates.io
// packages, so they meet the Java, Python, Go, JavaScript and Rust plugins'
// nodes. MODULE.bazel and WORKSPACE declarations are imports themselves.
//
// Starlark is read by internal/lang/starlark, not the vendored tree-sitter
// grammar (REQ-BAZEL-012).
package bazel

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemBazel      = "bazel"      // Bazel modules (Bazel Central Registry)
	ecosystemRepository = "bazel-repo" // WORKSPACE downloads named by their URL
	ecosystemStd        = "bazel-std"  // repositories Bazel itself provides
	ecosystemMaven      = "maven"
	ecosystemPyPI       = "pypi"
	ecosystemGo         = "go"
	ecosystemNPM        = "npm"
	ecosystemCrates     = "crates"
)

// File kinds, which Class names.
const (
	kindBuild     = "build"
	kindBzl       = "bzl"
	kindModule    = "module"
	kindWorkspace = "workspace"
)

// Implements: REQ-BAZEL-001
type Plugin struct{}

func (Plugin) Name() string { return "bazel" }
func (Plugin) Version() int { return 1 }

// fileKind is what a Starlark file of Bazel's is, by its name; "" is none.
func fileKind(p string) string {
	base := path.Base(p)
	switch {
	case base == "BUILD" || base == "BUILD.bazel":
		return kindBuild
	case base == "MODULE.bazel" || strings.HasSuffix(base, ".MODULE.bazel"):
		return kindModule
	case base == "WORKSPACE" || base == "WORKSPACE.bazel" || base == "WORKSPACE.bzlmod":
		return kindWorkspace
	case strings.HasSuffix(base, ".bzl"):
		return kindBzl
	}
	return ""
}

// Claims takes BUILD files, .bzl files, MODULE.bazel (and the segments it
// include()s) and WORKSPACE files, outside Bazel's output trees.
//
// Implements: REQ-BAZEL-001
func (Plugin) Claims(f *scan.File) bool {
	return !f.Binary && fileKind(f.Path) != "" && !skipped(f.Path)
}

// skipped reports whether a path is inside Bazel's output trees (bazel-bin,
// bazel-out, ...), which a checkout without git could hold as directories.
func skipped(p string) bool {
	for _, segment := range strings.Split(path.Dir(p), "/") {
		switch segment {
		case "bazel-bin", "bazel-out", "bazel-testlogs", "bazel-genfiles":
			return true
		}
	}
	return false
}

// Ecosystems: Bazel modules, WORKSPACE downloads, Bazel's own repositories, and
// the ecosystems module extensions install packages from (merged with the other
// plugins' islands of the same ids).
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemBazel, Name: "Bazel modules"},
		{ID: ecosystemRepository, Name: "Bazel repositories"},
		{ID: ecosystemStd, Name: "Bazel built-in repositories", Std: true},
		{ID: ecosystemMaven, Name: "Maven"},
		{ID: ecosystemPyPI, Name: "PyPI"},
		{ID: ecosystemGo, Name: "Go modules"},
		{ID: ecosystemNPM, Name: "npm"},
		{ID: ecosystemCrates, Name: "crates.io"},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Class is the file's kind; for a BUILD file also its directory, which names its
// targets (//dir:name).
//
// Implements: REQ-BAZEL-001
func (Plugin) Class(f *scan.File) string {
	k := fileKind(f.Path)
	if k == kindBuild {
		return k + ":" + path.Dir(f.Path)
	}
	return k
}

// Extract reads a file's declarations and the labels it names.
//
// Implements: REQ-BAZEL-002, REQ-BAZEL-003, REQ-BAZEL-004
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	return extract(fileKind(f.Path), path.Dir(f.Path), source), nil
}
