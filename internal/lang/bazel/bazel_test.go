package bazel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/lang/starlark"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is one Bzlmod workspace (MODULE.bazel with overrides, module
// extensions for Maven, pip, Go, npm and crates, a lock file and an include)
// holding a nested local module (api/) and a legacy WORKSPACE workspace (legacy/)
// with its own local_repository.
//
// Verifies: REQ-BAZEL-001, REQ-BAZEL-002, REQ-BAZEL-003, REQ-BAZEL-004, REQ-BAZEL-005
// Verifies: REQ-BAZEL-006, REQ-BAZEL-007, REQ-BAZEL-008, REQ-BAZEL-009, REQ-BAZEL-010
// Verifies: REQ-BAZEL-011
func TestFixture(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	cases := map[string]struct {
		imports map[string]lang.Target
		symbols map[string]string
	}{
		"BUILD.bazel": {
			imports: map[string]lang.Target{
				"//src:shop": {Local: "src/BUILD.bazel"},
			},
			symbols: map[string]string{
				"//:shop": "alias",
			},
		},
		"MODULE.bazel": {
			imports: map[string]lang.Target{
				"rules_go":                             {Ecosystem: "bazel", Package: "rules_go", Version: "0.50.1", Pinned: true},
				"bazel_skylib":                         {Ecosystem: "bazel", Package: "bazel_skylib", Version: "1.7.1", Pinned: true},
				"protobuf":                             {Ecosystem: "bazel", Package: "protobuf", Version: "27.3", Requested: "27.0", Pinned: true},
				"abseil-cpp":                           {Ecosystem: "bazel", Package: "abseil-cpp", Version: "4a2c63365eff8823a5221db86ef490e828306f9d", Requested: "20240116.2", Pinned: true, Origin: "https://github.com/abseil/abseil-cpp.git"},
				"googletest":                           {Ecosystem: "bazel", Package: "googletest", Version: "v1.15.0", Requested: "1.14.0", Pinned: true, Origin: "https://github.com/google/googletest/archive/refs/tags/v1.15.0.tar.gz"},
				"shop_api":                             {Local: "api/MODULE.bazel"},
				"rules_foo":                            {Ecosystem: "bazel", Package: "rules_foo", Floating: true},
				"rules_jvm_external":                   {Ecosystem: "bazel", Package: "rules_jvm_external", Version: "6.2", Pinned: true},
				"rules_python":                         {Ecosystem: "bazel", Package: "rules_python", Version: "0.34.0", Pinned: true},
				"gazelle":                              {Ecosystem: "bazel", Package: "gazelle", Version: "0.38.0", Pinned: true},
				"aspect_rules_js":                      {Ecosystem: "bazel", Package: "aspect_rules_js", Version: "2.0.1", Pinned: true},
				"rules_rust":                           {Ecosystem: "bazel", Package: "rules_rust", Version: "0.49.3", Pinned: true},
				"@rules_jvm_external//:extensions.bzl": {Ecosystem: "bazel", Package: "rules_jvm_external", Version: "6.2", Pinned: true},
				"//:maven_install.json":                {Local: "maven_install.json"},
				"com.google.guava:guava:32.0.0-jre":    {Ecosystem: "maven", Package: "com.google.guava:guava", Version: "32.1.2-jre", Requested: "32.0.0-jre", Pinned: true},
				"junit:junit:4.13.2":                   {Ecosystem: "maven", Package: "junit:junit", Version: "4.13.2", Pinned: true},
				"org.slf4j:slf4j-api:2.0.13":           {Ecosystem: "maven", Package: "org.slf4j:slf4j-api", Version: "2.0.13", Pinned: true},
				"@rules_python//python/extensions:pip.bzl":     {Ecosystem: "bazel", Package: "rules_python", Version: "0.34.0", Pinned: true},
				"//:requirements_lock.txt":                     {Local: "requirements_lock.txt"},
				"@gazelle//:extensions.bzl":                    {Ecosystem: "bazel", Package: "gazelle", Version: "0.38.0", Pinned: true},
				"//:go.mod":                                    {Local: "go.mod"},
				"golang.org/x/sys":                             {Ecosystem: "go", Package: "golang.org/x/sys", Version: "v0.22.0", Pinned: true},
				"@aspect_rules_js//npm:extensions.bzl":         {Ecosystem: "bazel", Package: "aspect_rules_js", Version: "2.0.1", Pinned: true},
				"//web:pnpm-lock.yaml":                         {Local: "web/pnpm-lock.yaml"},
				"@rules_rust//crate_universe:extension.bzl":    {Ecosystem: "bazel", Package: "rules_rust", Version: "0.49.3", Pinned: true},
				"//:Cargo.lock":                                {Local: "Cargo.lock"},
				"//:Cargo.toml":                                {Local: "Cargo.toml"},
				"anyhow":                                       {Ecosystem: "crates", Package: "anyhow", Version: "=1.0.86", Pinned: true},
				"@bazel_tools//tools/build_defs/repo:http.bzl": {Ecosystem: "bazel-std", Package: "bazel_tools"},
				"sample_data":                                  {Ecosystem: "bazel-repo", Package: "example.com/data/sample", Version: "1.2.0", Pinned: true},
				"//tools:extensions.bzl":                       {Local: "tools/extensions.bzl"},
				"//:deps.MODULE.bazel":                         {Local: "deps.MODULE.bazel"},
				"//tools:toolchain":                            {Local: "tools/BUILD.bazel"},
			},
			symbols: map[string]string{
				"shop": "module",
			},
		},
		"api/BUILD.bazel": {
			imports: map[string]lang.Target{
				"api.h": {Local: "api/api.h"},
			},
			symbols: map[string]string{
				"//api:api": "cc_library",
			},
		},
		"api/MODULE.bazel": {
			imports: map[string]lang.Target{},
			symbols: map[string]string{
				"shop_api": "module",
			},
		},
		"deps.MODULE.bazel": {
			imports: map[string]lang.Target{
				"rules_cc": {Ecosystem: "bazel", Package: "rules_cc", Version: "0.0.10", Requested: "0.0.9", Pinned: true},
			},
			symbols: map[string]string{},
		},
		"legacy/BUILD": {
			imports: map[string]lang.Target{
				"@io_bazel_rules_go//go:def.bzl":       {Ecosystem: "bazel-repo", Package: "github.com/bazelbuild/rules_go", Version: "v0.50.1", Pinned: true},
				"legacy.go":                            {Local: "legacy/legacy.go"},
				"@com_github_google_uuid//:uuid":       {Ecosystem: "go", Package: "github.com/google/uuid", Version: "v1.6.0", Pinned: true},
				"@com_google_absl//absl/strings":       {Ecosystem: "bazel-repo", Package: "github.com/abseil/abseil-cpp", Version: "4a2c63365eff8823a5221db86ef490e828306f9d", Pinned: true},
				"@com_github_gflags_gflags//:gflags":   {Ecosystem: "bazel-repo", Package: "github.com/gflags/gflags", Version: "v2.2.2"},
				"@floating//:lib":                      {Ecosystem: "bazel-repo", Package: "github.com/acme/floating", Version: "main", Floating: true},
				"@vendored//:lib":                      {Local: "legacy/third_party/vendored/BUILD"},
				"@zlib":                                {Ecosystem: "bazel-repo", Package: "zlib.net/zlib", Version: "1.3.1"},
				"@unpinned//:x":                        {Ecosystem: "bazel-repo", Package: "github.com/acme/unpinned", Version: "main", Floating: true},
				"@maven//:com_squareup_okhttp3_okhttp": {Ecosystem: "maven", Package: "com.squareup.okhttp3:okhttp", Version: "4.12.0", Pinned: true},
				"@legacy//:legacy.go":                  {Local: "legacy/legacy.go"},
			},
			symbols: map[string]string{
				"//legacy:legacy": "go_library",
			},
		},
		"legacy/WORKSPACE": {
			imports: map[string]lang.Target{
				"@bazel_tools//tools/build_defs/repo:http.bzl": {Ecosystem: "bazel-std", Package: "bazel_tools"},
				"@bazel_tools//tools/build_defs/repo:git.bzl":  {Ecosystem: "bazel-std", Package: "bazel_tools"},
				"//:deps.bzl":                        {Local: "legacy/deps.bzl"},
				"@rules_jvm_external//:defs.bzl":     {Ecosystem: "bazel", Package: "rules_jvm_external", Unresolved: true},
				"io_bazel_rules_go":                  {Ecosystem: "bazel-repo", Package: "github.com/bazelbuild/rules_go", Version: "v0.50.1", Pinned: true},
				"zlib":                               {Ecosystem: "bazel-repo", Package: "zlib.net/zlib", Version: "1.3.1"},
				"//third_party:zlib.BUILD":           {},
				"unpinned":                           {Ecosystem: "bazel-repo", Package: "github.com/acme/unpinned", Version: "main", Floating: true},
				"com_github_gflags_gflags":           {Ecosystem: "bazel-repo", Package: "github.com/gflags/gflags", Version: "v2.2.2"},
				"floating":                           {Ecosystem: "bazel-repo", Package: "github.com/acme/floating", Version: "main", Floating: true},
				"vendored":                           {Local: "legacy/third_party/vendored/WORKSPACE"},
				"com.squareup.okhttp3:okhttp:4.12.0": {Ecosystem: "maven", Package: "com.squareup.okhttp3:okhttp", Version: "4.12.0", Pinned: true},
			},
			symbols: map[string]string{
				"legacy": "workspace",
			},
		},
		"legacy/deps.bzl": {
			imports: map[string]lang.Target{
				"@bazel_gazelle//:deps.bzl":                     {Ecosystem: "bazel", Package: "bazel_gazelle", Unresolved: true},
				"@bazel_tools//tools/build_defs/repo:utils.bzl": {Ecosystem: "bazel-std", Package: "bazel_tools"},
				"@bazel_tools//tools/build_defs/repo:http.bzl":  {Ecosystem: "bazel-std", Package: "bazel_tools"},
				"com_github_google_uuid":                        {Ecosystem: "go", Package: "github.com/google/uuid", Version: "v1.6.0", Pinned: true},
				"com_google_absl":                               {Ecosystem: "bazel-repo", Package: "github.com/abseil/abseil-cpp", Version: "4a2c63365eff8823a5221db86ef490e828306f9d", Pinned: true},
			},
			symbols: map[string]string{
				"legacy_deps": "func",
			},
		},
		"legacy/third_party/vendored/BUILD": {
			imports: map[string]lang.Target{
				"lib.c": {Local: "legacy/third_party/vendored/lib.c"},
			},
			symbols: map[string]string{
				"//legacy/third_party/vendored:lib": "cc_library",
			},
		},
		"legacy/third_party/vendored/WORKSPACE": {
			imports: map[string]lang.Target{},
			symbols: map[string]string{
				"vendored": "workspace",
			},
		},
		"lib/BUILD": {
			imports: map[string]lang.Target{
				"lib.h":      {Local: "lib/lib.h"},
				"helpers.cc": {Local: "lib/helpers.cc"},
			},
			symbols: map[string]string{
				"//lib:lib":     "cc_library",
				"//lib:helpers": "cc_library",
			},
		},
		"src/BUILD.bazel": {
			imports: map[string]lang.Target{
				"@rules_cc//cc:defs.bzl":            {Ecosystem: "bazel", Package: "rules_cc", Version: "0.0.10", Requested: "0.0.9", Pinned: true},
				"@pypi//:requirements.bzl":          {Ecosystem: "bazel", Package: "rules_python", Version: "0.34.0", Pinned: true},
				"@rules_jvm_external//:defs.bzl":    {Ecosystem: "bazel", Package: "rules_jvm_external", Version: "6.2", Pinned: true},
				"//tools:defs.bzl":                  {Local: "tools/defs.bzl"},
				":local.bzl":                        {Local: "src/local.bzl"},
				"shop.cc":                           {Local: "src/shop.cc"},
				"detail/impl.h":                     {Local: "src/detail/impl.h"},
				"shop.h":                            {Local: "src/shop.h"},
				":util":                             {},
				"//lib":                             {Local: "lib/BUILD"},
				"//lib:helpers":                     {Local: "lib/BUILD"},
				"@//lib:helpers.cc":                 {Local: "lib/helpers.cc"},
				"@shop_repo//lib:lib":               {Local: "lib/BUILD"},
				"@com_google_protobuf//:protobuf":   {Ecosystem: "bazel", Package: "protobuf", Version: "27.3", Requested: "27.0", Pinned: true},
				"@abseil-cpp//absl/strings":         {Ecosystem: "bazel", Package: "abseil-cpp", Version: "4a2c63365eff8823a5221db86ef490e828306f9d", Requested: "20240116.2", Pinned: true, Origin: "https://github.com/abseil/abseil-cpp.git"},
				"@shop_api//:api":                   {Local: "api/BUILD.bazel"},
				"@@rules_go+//go/runfiles":          {Ecosystem: "bazel", Package: "rules_go", Version: "0.50.1", Pinned: true},
				"@bazel_tools//tools/cpp:toolchain": {Ecosystem: "bazel-std", Package: "bazel_tools"},
				"@unknown_repo//:thing":             {Ecosystem: "bazel", Package: "unknown_repo", Unresolved: true},
				"@shop_tools//:bin":                 {Local: "tools/extensions.bzl"},
				"@googletest//:gtest":               {Ecosystem: "bazel", Package: "googletest", Version: "v1.15.0", Requested: "1.14.0", Pinned: true, Origin: "https://github.com/google/googletest/archive/refs/tags/v1.15.0.tar.gz"},
				"shop_test.cc":                      {Local: "src/shop_test.cc"},
				":shop":                             {},
				"tool.py":                           {Local: "src/tool.py"},
				"requirement(requests)":             {Ecosystem: "pypi", Package: "requests", Version: "2.32.3", Pinned: true},
				"@pypi//pyyaml":                     {Ecosystem: "pypi", Package: "PyYAML", Version: "6.0.1", Pinned: true},
				"@pypi//typing_extensions:pkg":      {Ecosystem: "pypi", Package: "typing-extensions", Version: "4.12.2", Pinned: true},
				"@pypi//missing":                    {Ecosystem: "pypi", Package: "missing", Unresolved: true},
				"Shop.java":                         {Local: "src/Shop.java"},
				"@maven//:com_google_guava_guava":   {Ecosystem: "maven", Package: "com.google.guava:guava", Version: "32.1.2-jre", Requested: "32.0.0-jre", Pinned: true},
				"@maven//:org_slf4j_slf4j_api":      {Ecosystem: "maven", Package: "org.slf4j:slf4j-api", Version: "2.0.13", Pinned: true},
				"@maven//:not_declared":             {Ecosystem: "maven", Package: "not_declared", Unresolved: true},
				"artifact(junit:junit)":             {Ecosystem: "maven", Package: "junit:junit", Version: "4.13.2", Pinned: true},
				"shop.go":                           {Local: "src/shop.go"},
				"@com_github_pkg_errors//:errors":   {Ecosystem: "go", Package: "github.com/pkg/errors", Version: "v0.9.1", Pinned: true},
				"@org_golang_x_sys//unix":           {Ecosystem: "go", Package: "golang.org/x/sys", Version: "v0.22.0", Pinned: true},
				"lib.rs":                            {Local: "src/lib.rs"},
				"@crates//:serde":                   {Ecosystem: "crates", Package: "serde", Version: "1.0.204", Pinned: true},
				"@crates//:anyhow":                  {Ecosystem: "crates", Package: "anyhow", Version: "=1.0.86", Pinned: true},
			},
			symbols: map[string]string{
				"//src:shop":               "cc_library",
				"//src:util":               "cc_library",
				"//src:shop_test":          "cc_test",
				"//src:tool":               "py_binary",
				"//src:java":               "java_library",
				"//src:go_default_library": "go_library",
				"//src:rs":                 "rust_library",
				"//src:generated":          "shop_macro",
			},
		},
		"src/local.bzl": {
			imports: map[string]lang.Target{},
			symbols: map[string]string{
				"LOCAL": "var",
			},
		},
		"src/sub/BUILD.bazel": {
			imports: map[string]lang.Target{
				"sub.h":      {Local: "src/sub/sub.h"},
				"//src:util": {Local: "src/BUILD.bazel"},
			},
			symbols: map[string]string{
				"//src/sub:sub": "cc_library",
			},
		},
		"tools/BUILD.bazel": {
			imports: map[string]lang.Target{},
			symbols: map[string]string{
				"//tools:toolchain": "toolchain",
			},
		},
		"tools/defs.bzl": {
			imports: map[string]lang.Target{
				"@bazel_skylib//lib:paths.bzl": {Ecosystem: "bazel", Package: "bazel_skylib", Version: "1.7.1", Pinned: true},
				":private.bzl":                 {Local: "tools/private.bzl"},
				"//lib:helpers":                {Local: "lib/BUILD"},
			},
			symbols: map[string]string{
				"ShopInfo":        "provider",
				"VERSION":         "var",
				"_shop_rule_impl": "func",
				"shop_rule":       "rule",
				"shop_macro":      "func",
			},
		},
		"tools/extensions.bzl": {
			imports: map[string]lang.Target{},
			symbols: map[string]string{
				"_tools_impl": "func",
				"tools":       "module_extension",
			},
		},
		"tools/private.bzl": {
			imports: map[string]lang.Target{},
			symbols: map[string]string{
				"helper": "func",
			},
		},
		"web/BUILD.bazel": {
			imports: map[string]lang.Target{
				"@npm//:defs.bzl":      {Ecosystem: "bazel", Package: "aspect_rules_js", Version: "2.0.1", Pinned: true},
				"app.js":               {Local: "web/app.js"},
				":node_modules/lodash": {Ecosystem: "npm", Package: "lodash", Version: "4.17.21", Pinned: true},
				"@npm//@types/node":    {Ecosystem: "npm", Package: "@types/node", Version: "20.14.9", Pinned: true},
			},
			symbols: map[string]string{
				"//web:node_modules": "npm_link_all_packages",
				"//web:app":          "js_library",
			},
		},
	}
	for file, c := range cases {
		t.Run(file, func(t *testing.T) {
			langtest.CheckImports(t, results[file], c.imports)
			langtest.CheckSymbols(t, results[file], c.symbols)
		})
	}
	for file := range results {
		if _, ok := cases[file]; !ok {
			t.Errorf("%s: analyzed but not checked", file)
		}
	}
}

// Verifies: REQ-BAZEL-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"BUILD": true, "pkg/BUILD.bazel": true, "defs.bzl": true, "MODULE.bazel": true,
		"deps.MODULE.bazel": true, "WORKSPACE": true, "WORKSPACE.bazel": true, "WORKSPACE.bzlmod": true,
		"MODULE.bazel.lock": false, ".bazelrc": false, "BUILD.txt": false, "build": false,
		"bazel-out/k8/bin/x/BUILD": false, "x.star": false,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("Claims(%s) = %v, want %v", p, got, want)
		}
	}
	// A BUILD file names its targets by its directory: the same content elsewhere is
	// read again.
	source := []byte("cc_library(name = \"x\")\n")
	a := cache.Key("bazel", 1, lang.ClassOf(Plugin{}, &scan.File{Path: "a/BUILD"}), source)
	b := cache.Key("bazel", 1, lang.ClassOf(Plugin{}, &scan.File{Path: "b/BUILD"}), source)
	c := cache.Key("bazel", 1, lang.ClassOf(Plugin{}, &scan.File{Path: "a/WORKSPACE"}), source)
	if a == b || a == c {
		t.Error("BUILD files of different packages, or a BUILD and a WORKSPACE, share a cache key")
	}
}

// Verifies: REQ-BAZEL-004
func TestLabels(t *testing.T) {
	for s, want := range map[string]label{
		"//a/b:c":           {absolute: true, packageName: "a/b", target: "c"},
		"//a/b":             {absolute: true, packageName: "a/b", target: "b"},
		":c":                {target: "c"},
		"c.cc":              {target: "c.cc"},
		"sub/c.cc":          {target: "sub/c.cc"},
		"@r//a:c":           {repository: "r", hasRepository: true, absolute: true, packageName: "a", target: "c"},
		"@r":                {repository: "r", hasRepository: true, absolute: true, target: "r"},
		"@r//:r":            {repository: "r", hasRepository: true, absolute: true, target: "r"},
		"@//a:c":            {hasRepository: true, absolute: true, packageName: "a", target: "c"},
		"@@//a:c":           {hasRepository: true, canonical: true, absolute: true, packageName: "a", target: "c"},
		"@@rules_go+//go:x": {repository: "rules_go+", hasRepository: true, canonical: true, absolute: true, packageName: "go", target: "x"},
	} {
		got, ok := parseLabel(s)
		if !ok || got != want {
			t.Errorf("parseLabel(%q) = %+v, %v; want %+v", s, got, ok, want)
		}
	}
	for _, s := range []string{"", "$(location :x)", "@r:x", "//../x:y", "-lm"} {
		if _, ok := parseLabel(s); ok {
			t.Errorf("parseLabel(%q) accepted", s)
		}
	}
	for canonical, want := range map[string]string{"rules_go~0.50.1": "rules_go", "rules_go~": "rules_go", "rules_go+": "rules_go", "rules_go+ext+x": "rules_go", "abc": "abc"} {
		if got := moduleName(canonical); got != want {
			t.Errorf("moduleName(%s) = %s", canonical, got)
		}
	}
	for path, want := range map[string]string{
		"github.com/pkg/errors": "com_github_pkg_errors", "golang.org/x/sys": "org_golang_x_sys",
		"gopkg.in/yaml.v3": "in_gopkg_yaml_v3", "github.com/Azure/go-autorest": "com_github_azure_go_autorest",
	} {
		if got := goRepositoryName(path); got != want {
			t.Errorf("goRepoName(%s) = %s, want %s", path, got, want)
		}
	}
}

// Verifies: REQ-BAZEL-005
func TestGlob(t *testing.T) {
	for _, c := range []struct {
		pattern, name string
		want          bool
	}{
		{"*.cc", "a.cc", true}, {"*.cc", "d/a.cc", false}, {"**/*.h", "a.h", true},
		{"**/*.h", "d/e/a.h", true}, {"d/**", "d/e/f", true}, {"d/**", "e/f", false},
		{"**", "x", true}, {"a/**/b/*.txt", "a/x/y/b/c.txt", true}, {"a/**/b/*.txt", "a/b/c.txt", true},
		{"*.cc", "a.h", false},
	} {
		if got := globMatch(c.pattern, c.name); got != c.want {
			t.Errorf("globMatch(%q, %q) = %v", c.pattern, c.name, got)
		}
	}
	// Twelve ** do not make matching exponential.
	globMatch(strings.Repeat("**/", 12)+"x", strings.Repeat("a/", 30)+"y")
}

// Verifies: REQ-BAZEL-007, REQ-BAZEL-008
func TestVersionsAndNames(t *testing.T) {
	ordered := []string{"0.0.10-rc1", "0.0.10", "0.0.10.bcr.1", "0.0.11", "1.2.0", "1.10.0", "20240116.2"}
	for i := range ordered {
		for j := range ordered {
			if got := lessVersion(ordered[i], ordered[j]); got != (i < j) {
				t.Errorf("lessVersion(%s, %s) = %v", ordered[i], ordered[j], got)
			}
		}
	}
	for u, want := range map[string][2]string{
		"https://github.com/bazelbuild/rules_go/releases/download/v0.50.1/rules_go-v0.50.1.zip": {"github.com/bazelbuild/rules_go", "v0.50.1"},
		"https://github.com/abseil/abseil-cpp/archive/refs/tags/20240116.2.tar.gz":              {"github.com/abseil/abseil-cpp", "20240116.2"},
		"https://mirror.bazel.build/github.com/madler/zlib/archive/v1.3.1.tar.gz":               {"github.com/madler/zlib", "v1.3.1"},
		"https://gitlab.com/acme/lib/-/archive/v2.0/lib-v2.0.tar.gz":                            {"gitlab.com/acme/lib", "v2.0"},
		"https://zlib.net/zlib-1.3.1.tar.gz":                                                    {"zlib.net/zlib", "1.3.1"},
		"https://example.com/downloads/tool.tar.gz?raw=true":                                    {"example.com/downloads/tool", ""},
	} {
		if name, reference, _ := archiveName(u); name != want[0] || reference != want[1] {
			t.Errorf("archiveName(%s) = %s, %s", u, name, reference)
		}
	}
}

// Verifies: REQ-BAZEL-008, REQ-BAZEL-009
func TestDependencies(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	got := r.Dependencies(lang.Target{Ecosystem: "maven", Package: "com.google.guava:guava", Version: "32.1.2-jre"})
	want := []lang.Target{{Ecosystem: "maven", Package: "com.google.guava:failureaccess", Version: "1.0.1", Pinned: true}}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("guava needs %+v, want %+v", got, want)
	}

	// Lock files before Bazel 7.2 keep the resolved module graph.
	directory := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("MODULE.bazel", `bazel_dep(name = "rules_go", version = "0.41.0")`+"\n")
	write("MODULE.bazel.lock", `{"lockFileVersion": 3, "moduleDepGraph": {
		"<root>": {"name": "", "version": "", "deps": {"rules_go": "rules_go@0.42.0"}},
		"rules_go@0.42.0": {"name": "rules_go", "version": "0.42.0", "deps": {"bazel_skylib": "bazel_skylib@1.4.1", "bazel_tools": "bazel_tools@_"}},
		"bazel_skylib@1.4.1": {"name": "bazel_skylib", "version": "1.4.1", "deps": {}}}}`)
	files, err := scan.Scan(context.Background(), directory, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	r = newResolver(directory, files)
	dependency := r.Resolve("MODULE.bazel", lang.RawImport{Spec: "rules_go", Module: "rules_go", Name: importDependency})
	if want := (lang.Target{Ecosystem: "bazel", Package: "rules_go", Version: "0.42.0", Requested: "0.41.0", Pinned: true}); dependency != want {
		t.Errorf("rules_go = %+v, want %+v", dependency, want)
	}
	got = r.Dependencies(dependency)
	if len(got) != 1 || got[0] != (lang.Target{Ecosystem: "bazel", Package: "bazel_skylib", Version: "1.4.1", Pinned: true}) {
		t.Errorf("rules_go needs %+v", got)
	}
}

// Every prefix of every fixture file, and pathological inputs, are read without a
// panic or a runaway.
//
// Verifies: REQ-BAZEL-012
func TestTruncated(t *testing.T) {
	for _, f := range langtest.Files(t, "testdata/repo") {
		if fileKind(f.Path) == "" {
			continue
		}
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			t.Fatal(err)
		}
		for n := 0; n <= len(source); n++ {
			if extraction, err := (Plugin{}).Extract(f, source[:n]); err != nil || extraction == nil {
				t.Fatalf("%s[:%d]: %v", f.Path, n, err)
			}
		}
	}
	for _, s := range []string{
		strings.Repeat("(", 100000), strings.Repeat("[", 100000), strings.Repeat("{", 100000),
		strings.Repeat("f(", 50000) + strings.Repeat(")", 50000), strings.Repeat("-", 100000),
		strings.Repeat("not ", 50000), strings.Repeat("a if b else ", 30000), strings.Repeat(`"`, 100000),
		strings.Repeat(`"""`, 30000), strings.Repeat("r'", 50000), strings.Repeat("\\\n", 50000),
		strings.Repeat("def f():\n ", 20000), strings.Repeat("x = [\n", 50000), strings.Repeat("a.", 100000),
		strings.Repeat("lambda x: ", 30000), strings.Repeat("glob([", 30000),
		strings.Repeat("cc_library(name = \"x\", deps = [", 20000),
		strings.Repeat("select({\"a\": ", 20000),
	} {
		for _, kind := range []string{kindBuild, kindBzl, kindModule, kindWorkspace} {
			extract(kind, ".", []byte(s))
		}
		starlark.Parse([]byte(s))
	}
}
