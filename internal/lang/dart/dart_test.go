package dart

import (
	"maps"
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// A pub workspace (packages/shop_core and packages/shop_utils) beside a Flutter app
// with its own pubspec.lock that depends on shop_core by path: relative, self and
// workspace package: URIs, a configurable import's every branch, part and part of,
// dart: libraries, Flutter's SDK packages, hosted packages pinned by the lock (a
// custom server's too), a git package locked to a commit, one on a branch, a bare
// version, `any` and no constraint, an override, a locked package nothing declares
// and one nothing declares or locks; and the pubspecs' dependencies and workspace
// members as imports.
//
// Verifies: REQ-DART-001, REQ-DART-002, REQ-DART-003, REQ-DART-004, REQ-DART-005
// Verifies: REQ-DART-006, REQ-DART-007, REQ-DART-008
func TestWorkspaceAndApp(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	imports := map[string]map[string]lang.Target{
		"app/lib/main.dart": {
			"import 'dart:async'":                                          {Ecosystem: ecoStd, Package: "dart:async"},
			"import 'dart:io'":                                             {Ecosystem: ecoStd, Package: "dart:io"},
			"import 'package:flutter/material.dart'":                       {Ecosystem: ecoFlutter, Package: "flutter"},
			"import 'package:http/http.dart'":                              {Ecosystem: ecoPub, Package: "http", Version: "1.2.1", Requested: "^1.2.0", Pinned: true},
			"import 'package:shop_app/src/model.dart'":                     {Local: "app/lib/src/model.dart"},
			"import 'package:shop_core/shop_core.dart'":                    {Local: "packages/shop_core/lib/shop_core.dart"},
			"import 'package:path/path.dart'":                              {Ecosystem: ecoPub, Package: "path", Version: "1.9.0", Pinned: true},
			"import 'package:missing/missing.dart'":                        {Ecosystem: ecoPub, Package: "missing", Unresolved: true},
			"import 'package:provider/provider.dart'":                      {Ecosystem: ecoPub, Package: "provider", Version: "6.1.2", Pinned: true},
			"import 'package:tracing/tracing.dart'":                        {Ecosystem: ecoPub, Package: "tracing", Version: "0.3.0", Pinned: true, Origin: "https://github.com/acme/tracing.git"},
			"import 'package:nightly/nightly.dart'":                        {Ecosystem: ecoPub, Package: "nightly", Version: "main", Floating: true, Origin: "https://github.com/acme/nightly.git"},
			"import 'package:private_kit/private_kit.dart'":                {Ecosystem: ecoPub, Package: "private_kit", Version: "2.1.0", Requested: "^2.0.0", Pinned: true},
			"import 'package:flutter_gen/gen_l10n/app_localizations.dart'": {},
			"import 'src/platform.dart'":                                   {Local: "app/lib/src/platform.dart"},
			"import 'src/platform_io.dart' if (dart.library.io)":           {Local: "app/lib/src/platform_io.dart"},
			"import 'src/platform_web.dart' if (dart.library.js_interop)":  {Local: "app/lib/src/platform_web.dart"},
			"import 'https://example.com/remote.dart'":                     {},
			"export 'src/model.dart'":                                      {Local: "app/lib/src/model.dart"},
			"part 'main.g.dart'":                                           {},
		},
		"app/lib/src/model.dart": {
			"import 'package:collection/collection.dart'": {Ecosystem: ecoPub, Package: "collection", Version: "1.18.0", Requested: "any", Pinned: true},
			"import 'package:intl/intl.dart'":             {Ecosystem: ecoPub, Package: "intl", Floating: true},
			"import 'package:meta/meta.dart'":             {Ecosystem: ecoPub, Package: "meta", Version: "1.11.0", Pinned: true},
			"part 'model.part.dart'":                      {Local: "app/lib/src/model.part.dart"},
		},
		"app/lib/src/model.part.dart": {
			"part of 'model.dart'": {Local: "app/lib/src/model.dart"},
		},
		"app/lib/src/platform.dart": {},
		"app/lib/src/platform_io.dart": {
			"import 'dart:io'": {Ecosystem: ecoStd, Package: "dart:io"},
		},
		"app/lib/src/platform_web.dart": {
			"import 'dart:js_interop'": {Ecosystem: ecoStd, Package: "dart:js_interop"},
		},
		"app/pubspec.yaml": {
			"flutter (sdk: flutter)":                  {Ecosystem: ecoFlutter, Package: "flutter"},
			"flutter_localizations (sdk: flutter)":    {Ecosystem: ecoFlutter, Package: "flutter_localizations"},
			"http: ^1.2.0":                            {Ecosystem: ecoPub, Package: "http", Version: "1.2.1", Requested: "^1.2.0", Pinned: true},
			"provider: 6.1.2":                         {Ecosystem: ecoPub, Package: "provider", Version: "6.1.2", Pinned: true},
			"collection: any":                         {Ecosystem: ecoPub, Package: "collection", Version: "1.18.0", Requested: "any", Pinned: true},
			"intl: any":                               {Ecosystem: ecoPub, Package: "intl", Floating: true},
			"shop_core (path: ../packages/shop_core)": {Local: "packages/shop_core/pubspec.yaml"},
			"tracing (git: https://github.com/acme/tracing.git 0123456789abcdef0123456789abcdef01234567)": {Ecosystem: ecoPub, Package: "tracing", Version: "0.3.0", Pinned: true, Origin: "https://github.com/acme/tracing.git"},
			"nightly (git: https://github.com/acme/nightly.git main)":                                     {Ecosystem: ecoPub, Package: "nightly", Version: "main", Floating: true, Origin: "https://github.com/acme/nightly.git"},
			"private_kit: ^2.0.0 (hosted: https://pub.acme.test)":                                         {Ecosystem: ecoPub, Package: "private_kit", Version: "2.1.0", Requested: "^2.0.0", Pinned: true},
			"flutter_test (sdk: flutter)":                                                                 {Ecosystem: ecoFlutter, Package: "flutter_test"},
			"lints: >=3.0.0 <4.0.0":                                                                       {Ecosystem: ecoPub, Package: "lints", Version: "3.0.0", Requested: ">=3.0.0 <4.0.0", Pinned: true},
			"override meta: 1.11.0":                                                                       {Ecosystem: ecoPub, Package: "meta", Version: "1.11.0", Pinned: true},
		},
		"app/test/widget_test.dart": {
			"import 'package:flutter_test/flutter_test.dart'": {Ecosystem: ecoFlutter, Package: "flutter_test"},
			"import 'package:shop_app/main.dart'":             {Local: "app/lib/main.dart"},
			"import 'package:lints/lints.dart'":               {Ecosystem: ecoPub, Package: "lints", Version: "3.0.0", Requested: ">=3.0.0 <4.0.0", Pinned: true},
		},
		"packages/shop_core/lib/shop_core.dart": {
			"import 'package:collection/collection.dart'": {Ecosystem: ecoPub, Package: "collection", Version: "1.18.0", Requested: "^1.18.0", Pinned: true},
			"import 'package:shop_utils/shop_utils.dart'": {Local: "packages/shop_utils/lib/shop_utils.dart"},
			"export 'src/cart.dart'":                      {Local: "packages/shop_core/lib/src/cart.dart"},
		},
		"packages/shop_core/lib/src/cart.dart": {
			"import '../shop_core.dart'": {Local: "packages/shop_core/lib/shop_core.dart"},
		},
		"packages/shop_core/pubspec.yaml": {
			"collection: ^1.18.0": {Ecosystem: ecoPub, Package: "collection", Version: "1.18.0", Requested: "^1.18.0", Pinned: true},
			"shop_utils: ^1.0.0":  {Local: "packages/shop_utils/pubspec.yaml"},
		},
		"packages/shop_utils/lib/shop_utils.dart": {},
		"packages/shop_utils/pubspec.yaml":        {},
		"pubspec.yaml": {
			"workspace: packages/shop_core":  {Local: "packages/shop_core/pubspec.yaml"},
			"workspace: packages/shop_utils": {Local: "packages/shop_utils/pubspec.yaml"},
			"lints: ^3.0.0":                  {Ecosystem: ecoPub, Package: "lints", Version: "3.0.0", Requested: "^3.0.0", Pinned: true},
		},
	}
	for file, want := range imports {
		langtest.CheckImports(t, res[file], want)
	}
	if len(res) != len(imports) {
		t.Errorf("analyzed %d files, want %d", len(res), len(imports))
	}
	symbols := map[string]map[string]string{
		"app/lib/main.dart":                       {"main": "func", "ShopApp": "class", "ShopApp.new": "constructor", "ShopApp.build": "method"},
		"app/lib/src/model.dart":                  {"Json": "typedef", "Listener": "typedef", "currency": "const", "formatter": "var", "counter": "var", "describe": "var", "total": "func", "save": "func", "greeting": "property", "Entity": "class", "Entity.new": "constructor", "Entity.id": "field", "Product": "class", "Product.new": "constructor", "Product.free": "constructor", "Product.fromJson": "constructor", "Product.zero": "const", "Product.cents": "field", "Product.tags": "field", "Product.counts": "field", "Product.onChange": "field", "Product.compareTo": "method", "Product.operator==": "method", "Product.operator[]": "method", "Product.operator[]=": "method", "Product.label": "property", "Product.pair": "method", "Priced": "mixin", "Priced.cents": "property", "Priced.price": "method", "Tracked": "class", "Shape": "class", "Status": "enum", "Status.new": "constructor", "Status.code": "field", "Status.isActive": "property", "Plain": "enum", "ProductList": "extension", "ProductList.cents": "property", "ProductId": "extension type", "ProductId.parse": "constructor", "ProductId.isEmpty": "property"},
		"app/lib/src/model.part.dart":             {"describeProduct": "func"},
		"app/lib/src/platform.dart":               {"platform": "func"},
		"app/lib/src/platform_io.dart":            {"platform": "func"},
		"app/lib/src/platform_web.dart":           {"platform": "func"},
		"app/pubspec.yaml":                        {},
		"app/test/widget_test.dart":               {"main": "func"},
		"packages/shop_core/lib/shop_core.dart":   {"sortedSlugs": "func"},
		"packages/shop_core/lib/src/cart.dart":    {"Cart": "class", "Cart.items": "field", "Cart.slugs": "property"},
		"packages/shop_core/pubspec.yaml":         {},
		"packages/shop_utils/lib/shop_utils.dart": {"slug": "func"},
		"packages/shop_utils/pubspec.yaml":        {},
		"pubspec.yaml":                            {},
	}
	for file, want := range symbols {
		langtest.CheckSymbols(t, res[file], want)
	}
}

// melos links the packages its globs select, so a hosted constraint on one of them
// is the local package; a package the globs ignore takes it from pub.
//
// Verifies: REQ-DART-004
func TestMelosPackagesAreLocal(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/melos")
	imports := map[string]map[string]lang.Target{
		"examples/demo/lib/main.dart": {
			"import 'package:a/a.dart'": {Local: "packages/a/lib/a.dart"},
		},
		"examples/demo/pubspec.yaml": {
			"a: ^1.0.0": {Local: "packages/a/pubspec.yaml"},
		},
		"examples/legacy/lib/legacy.dart": {
			"import 'package:a/a.dart'": {Ecosystem: ecoPub, Package: "a", Version: "^1.0.0"},
		},
		"examples/legacy/pubspec.yaml": {
			"a: ^1.0.0": {Ecosystem: ecoPub, Package: "a", Version: "^1.0.0"},
		},
		"packages/a/lib/a.dart": {
			"import 'package:b/b.dart'": {Local: "packages/b/lib/b.dart"},
			"import 'package:c/c.dart'": {Ecosystem: ecoPub, Package: "c", Version: "^1.0.0"},
		},
		"packages/a/pubspec.yaml": {
			"b: ^1.0.0": {Local: "packages/b/pubspec.yaml"},
			"c: ^1.0.0": {Ecosystem: ecoPub, Package: "c", Version: "^1.0.0"},
		},
		"packages/b/lib/b.dart":   {},
		"packages/b/pubspec.yaml": {},
	}
	for file, want := range imports {
		langtest.CheckImports(t, res[file], want)
	}
}

// Dart sources and pubspecs are claimed; what pub generates under .dart_tool is not.
//
// Verifies: REQ-DART-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"lib/main.dart":                          true,
		"lib/model.g.dart":                       true,
		"pubspec.yaml":                           true,
		"app/pubspec.yaml":                       true,
		"pubspec.lock":                           false,
		"analysis_options.yaml":                  false,
		".dart_tool/package_config.json":         false,
		".dart_tool/flutter_gen/gen_l10n/l.dart": false,
		"app/.dart_tool/build/entrypoint.dart":   false,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("%s: claimed %v, want %v", p, got, want)
		}
	}
}

// A bare version pins; a caret, a range, any and no constraint float.
//
// Verifies: REQ-DART-008
func TestPubPinned(t *testing.T) {
	for c, want := range map[string]bool{
		"1.2.3": true, "1.2.3+4": true, "2.0.0-dev.1": true,
		"^1.2.3": false, ">=1.0.0 <2.0.0": false, "any": false, "": false, "1.2": false,
	} {
		if got := pubPinned(c); got != want {
			t.Errorf("%q: pinned %v, want %v", c, got, want)
		}
	}
}

// Strings hide what looks like code: quotes and braces inside an interpolation, a
// raw string's `${`, a triple-quoted string over several lines; nested block
// comments end where the outer one does; a condition comparing with == is kept.
//
// Verifies: REQ-DART-002, REQ-DART-003
func TestScannerSkipsStringsAndComments(t *testing.T) {
	src := `import 'a.dart' if (dart.library.io == 'true') 'b.dart';
/* outer /* inner */ still a comment: class NotAClass {} */
const s = 'x ${ {'k': '}'}['k'] } y';
const r = r'${';
const m = """
class AlsoNot {}
""";
class Real {
  String f() => "$s ${s.length}";
}
`
	ex, err := (Plugin{}).Extract(&scan.File{Path: "lib/x.dart"}, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var specs []string
	for _, im := range ex.Imports {
		specs = append(specs, im.Spec)
	}
	if want := []string{"import 'a.dart'", "import 'b.dart' if (dart.library.io == 'true')"}; !slices.Equal(specs, want) {
		t.Errorf("imports %q, want %q", specs, want)
	}
	got := map[string]string{}
	for _, s := range ex.Symbols {
		got[s.Name] = s.Kind
	}
	want := map[string]string{"s": "const", "r": "const", "m": "const", "Real": "class", "Real.f": "method"}
	if !maps.Equal(got, want) {
		t.Errorf("symbols %v, want %v", got, want)
	}
}

// A byte order mark does not hide the first directive.
//
// Verifies: REQ-DART-002
func TestByteOrderMark(t *testing.T) {
	ex, _ := (Plugin{}).Extract(&scan.File{Path: "lib/x.dart"}, []byte("\xef\xbb\xbfimport 'y.dart';\n"))
	if len(ex.Imports) != 1 || ex.Imports[0].Module != "y.dart" {
		t.Errorf("imports %+v, want y.dart", ex.Imports)
	}
}
