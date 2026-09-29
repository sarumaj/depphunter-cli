package haxe

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is a haxelib library "shop" (haxelib.json, classPath src) built
// by build.hxml (-cp, -lib with and without versions, --library, -main,
// -resource, --macro, --each/--next sections, a root module and an included
// common.hxml). src/shop/Main.hx imports modules of the repository (a sub-type,
// a package wildcard, a module's fields, a default-package module on another
// class path), the standard library, declared libraries, a library installed in
// the local haxelib repository .haxelib/ (format, pixels), and hides fake
// imports in comments, strings, an interpolation and a regular expression.
// game/ is a lix project (.haxerc, haxe_libraries/ pinning by haxelib version,
// git commit, tag and branch, and a library in development inside the
// repository); app/ an OpenFL project (Project.xml including shared.xml);
// docs/project.xml is another tool's file of that name.

func std(name string) lang.Target { return lang.Target{Ecosystem: ecosystemStd, Package: name} }

var (
	openfl92   = lang.Target{Ecosystem: ecosystemHaxelib, Package: "openfl", Version: "9.2.0", Pinned: true}
	tinkCore   = lang.Target{Ecosystem: ecosystemHaxelib, Package: "tink_core", Floating: true}
	tinkCore21 = lang.Target{Ecosystem: ecosystemHaxelib, Package: "tink_core", Version: "2.1.1", Pinned: true}
	unittest   = lang.Target{Ecosystem: ecosystemHaxelib, Package: "tink_unittest", Version: "28ed02beb6986bfff15073c5a9b000dc3f3fcc11", Pinned: true}
	testrunner = lang.Target{Ecosystem: ecosystemHaxelib, Package: "tink_testrunner", Version: "v0.9.0"}
	acmeapi    = lang.Target{Ecosystem: ecosystemHaxelib, Package: "acmeapi", Version: "main", Floating: true, Origin: "https://git.acme.dev/acmeapi.git"}
	format     = lang.Target{Ecosystem: ecosystemHaxelib, Package: "format", Version: "3.5.0", Floating: true}
	thxCore    = lang.Target{Ecosystem: ecosystemHaxelib, Package: "thx.core", Version: "0.44.0", Pinned: true}
	utest      = lang.Target{Ecosystem: ecosystemHaxelib, Package: "utest", Version: "1.13.2", Pinned: true}
	hxnodejs   = lang.Target{Ecosystem: ecosystemHaxelib, Package: "hxnodejs", Floating: true}
)

var importHx = lang.Target{Local: "src/import.hx"}

// analyze runs the plugin over the fixture with nothing installed outside it.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Setenv("HAXELIB_PATH", t.TempDir())
	t.Setenv("HAXE_LIBCACHE", t.TempDir())
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-HAXE-002, REQ-HAXE-004, REQ-HAXE-006, REQ-HAXE-007, REQ-HAXE-008, REQ-HAXE-011
func TestImports(t *testing.T) {
	results := analyze(t)
	langtest.CheckImports(t, results["src/shop/Main.hx"], map[string]lang.Target{
		"shop.model.Cart":                        {Local: "src/shop/model/Cart.hx"},
		"shop.model.Cart.CartItem":               {Local: "src/shop/model/Cart.hx"}, // a sub-type of the module
		"shop.util.* (src/shop/util/Money.hx)":   {Local: "src/shop/util/Money.hx"}, // every module of the package
		"shop.util.* (src/shop/util/Strings.hx)": {Local: "src/shop/util/Strings.hx"},
		"shop.util.Money.*":                      {Local: "src/shop/util/Money.hx"}, // the module's fields
		"haxe.Json":                              std("haxe"),
		"haxe.ds.StringMap":                      std("haxe.ds"),
		"sys.io.File":                            std("sys.io"), // import ... as F
		"StringTools":                            std("StringTools"),
		"Lambda":                                 std("Lambda"), // using
		"shop.util.Strings":                      {Local: "src/shop/util/Strings.hx"},
		"openfl.display.Sprite":                  openfl92,
		"tink.core.Future":                       tinkCore,
		"thx.Arrays":                             thxCore,                                                                            // the table's thx -> thx.core, declared
		"format.png.Reader":                      format,                                                                             // installed in .haxelib/, declared
		"gfx.Canvas":                             {Ecosystem: ecosystemHaxelib, Package: "pixels", Version: "1.0.0", Floating: true}, // installed only
		"js.node.Fs":                             hxnodejs,                                                                           // taken out of std by the table
		"haxe.ui.Toolkit":                        {Ecosystem: ecosystemHaxelib, Package: "haxeui-core", Unresolved: true},
		"mystery.Thing":                          {Ecosystem: ecosystemHaxelib, Package: "mystery", Unresolved: true},
		"shop.Gone":                              {}, // the repository's own package, missing
		"std.Math":                               std("Math"),
		"Config":                                 {Local: "shared/Config.hx"},
		"js.Browser":                             std("js"), // every #if branch counts
		"cpp.Lib":                                std("cpp"),
		"sys.FileSystem":                         std("sys"),
		"haxe.crypto.Md5":                        std("haxe.crypto"), // qualified names in code
		"flixel.FlxG":                            {},                 // in code, not declared here
		"flash.display.Sprite":                   std("flash.display"),
		"import.hx (src/import.hx)":              importHx, // what it imports applies here
	})
	langtest.CheckImports(t, results["src/shop/model/Cart.hx"], map[string]lang.Target{
		"shop.util.Money":           {Local: "src/shop/util/Money.hx"},
		"import.hx (src/import.hx)": importHx,
	})
	langtest.CheckImports(t, results["src/import.hx"], map[string]lang.Target{
		"shop.util.Strings": {Local: "src/shop/util/Strings.hx"},
	})
	langtest.CheckImports(t, results["test/TestAll.hx"], map[string]lang.Target{
		"utest.Assert":    utest,
		"shop.model.Cart": {Local: "src/shop/model/Cart.hx"},
	})
	langtest.CheckImports(t, results["game/src/Game.hx"], map[string]lang.Target{
		"tink.core.Future":       tinkCore21, // lix pins it
		"tink.unit.Assert":       unittest,
		"tink.testrunner.Runner": testrunner,
		"mylib.Thing":            {Local: "game/libs/mylib/src/mylib/Thing.hx"},
		"acmeapi.Client":         acmeapi,
	})
	langtest.CheckImports(t, results["app/source/app/Main.hx"], map[string]lang.Target{
		"flixel.FlxG":           {Ecosystem: ecosystemHaxelib, Package: "flixel", Floating: true},
		"openfl.display.Sprite": {Ecosystem: ecosystemHaxelib, Package: "openfl", Version: "9.3.0", Pinned: true}, // the nearest project file
		"motion.Actuate":        {Ecosystem: ecosystemHaxelib, Package: "actuate", Floating: true},                // declared in the included shared.xml
	})
	langtest.CheckImports(t, results["src/shop/macros/Build.hx"], map[string]lang.Target{
		"haxe.macro.Context":        std("haxe.macro"),
		"import.hx (src/import.hx)": importHx, // src/ is its class path
	})
}

// Verifies: REQ-HAXE-005, REQ-HAXE-006
func TestManifests(t *testing.T) {
	results := analyze(t)
	langtest.CheckImports(t, results["build.hxml"], map[string]lang.Target{
		"-cp src":                            {Local: "src"},
		"--class-path shared":                {Local: "shared"},
		"-lib openfl:9.2.0":                  openfl92,
		"-lib tink_core":                     tinkCore,
		"--library hxnodejs":                 hxnodejs,
		"-main shop.Main":                    {Local: "src/shop/Main.hx"},
		"-resource assets/banner.txt@banner": {Local: "assets/banner.txt"},
		"--macro shop.macros.Build":          {Local: "src/shop/macros/Build.hx"},
		"-lib utest:1.13.2":                  utest,
		"-cp test":                           {Local: "test"},
		"shop.util.Money":                    {Local: "src/shop/util/Money.hx"}, // a root module
		"common.hxml":                        {Local: "common.hxml"},
		"-lib hxcpp":                         {Ecosystem: ecosystemHaxelib, Package: "hxcpp", Floating: true},
	})
	langtest.CheckImports(t, results["common.hxml"], map[string]lang.Target{"-lib format": format})
	langtest.CheckImports(t, results["haxelib.json"], map[string]lang.Target{
		"lime":          {Ecosystem: ecosystemHaxelib, Package: "lime", Floating: true},
		"openfl":        openfl92,
		"thx.core":      thxCore,
		"fancy":         {Ecosystem: ecosystemHaxelib, Package: "fancy", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true, Origin: "https://git.acme.dev/fancy.git"},
		"tagged":        {Ecosystem: ecosystemHaxelib, Package: "tagged", Version: "v1.2.0"},
		"branchy":       {Ecosystem: ecosystemHaxelib, Package: "branchy", Version: "main", Floating: true},
		"classPath src": {Local: "src"},
	})
	langtest.CheckImports(t, results["game/build.hxml"], map[string]lang.Target{
		"-cp src":            {Local: "game/src"},
		"-lib tink_core":     tinkCore21,
		"-lib tink_unittest": unittest,
		"-lib mylib":         {Local: "game/libs/mylib/src"}, // in development in the repository
		"-main Game":         {Local: "game/src/Game.hx"},
	})
	for file, want := range map[string]lang.Target{
		"game/haxe_libraries/tink_core.hxml":       tinkCore21,
		"game/haxe_libraries/tink_unittest.hxml":   unittest,
		"game/haxe_libraries/tink_testrunner.hxml": testrunner,
		"game/haxe_libraries/acmeapi.hxml":         acmeapi,
		"game/haxe_libraries/mylib.hxml":           {Local: "game/libs/mylib/src"},
	} {
		name := strings.TrimSuffix(filepath.Base(file), ".hxml")
		langtest.CheckImports(t, results[file], map[string]lang.Target{name: want})
	}
	langtest.CheckImports(t, results["app/Project.xml"], map[string]lang.Target{
		"main app.Main":      {Local: "app/source/app/Main.hx"},
		"source source":      {Local: "app/source"},
		"classpath extra":    {Local: "app/extra"},
		"openfl":             {Ecosystem: ecosystemHaxelib, Package: "openfl", Version: "9.3.0", Pinned: true},
		"flixel":             {Ecosystem: ecosystemHaxelib, Package: "flixel", Floating: true},
		"include shared.xml": {Local: "app/shared.xml"},
		"howler":             {Ecosystem: ecosystemHaxelib, Package: "howler", Version: "2.2.0", Pinned: true},
	})
}

// Verifies: REQ-HAXE-006
func TestPinRule(t *testing.T) {
	for v, want := range map[string]lang.Target{
		"":                           {Floating: true},
		"4.3.0":                      {Version: "4.3.0", Pinned: true},
		"git":                        {Version: "git", Floating: true},
		"git:https://github.com/a/b": {Floating: true},
		"git:https://github.com/a/b#0123456789abcdef0123456789abcdef01234567": {Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true},
		"git:https://github.com/a/b#2.0.1":                                    {Version: "2.0.1"},
		"git:https://git.acme.dev/b.git#develop":                              {Version: "develop", Floating: true, Origin: "https://git.acme.dev/b.git"},
	} {
		var got lang.Target
		pinRule(&got, v)
		if got != want {
			t.Errorf("%q: %+v, want %+v", v, got, want)
		}
	}
	for url, want := range map[string]lixLibrary{
		"haxelib:/format#3.7.0": {version: "3.7.0", pinned: true},
		"haxelib:ansi#1.0.0":    {version: "1.0.0", pinned: true},
		"gh://github.com/a/b#0123456789abcdef0123456789abcdef01234567": {version: "0123456789abcdef0123456789abcdef01234567", pinned: true},
		"gh://github.com/a/b#v1.0.0":                                   {version: "v1.0.0"},
		"gl://gitlab.com/a/b#main":                                     {version: "main", floating: true},
		"https://example.org/b-1.0.0.zip":                              {version: "1.0.0", origin: "https://example.org/b-1.0.0.zip"},
		"":                                                             {version: "1.0.0", floating: true},
	} {
		var got lixLibrary
		readInstall(&got, url, "1.0.0")
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: %+v, want %+v", url, got, want)
		}
	}
}

// Verifies: REQ-HAXE-003
func TestSymbols(t *testing.T) {
	results := analyze(t)
	langtest.CheckSymbols(t, results["src/shop/Main.hx"], map[string]string{
		"Main": "class", "Main.new": "method", "Main.main": "method", "Main.helper": "method",
		"Extra": "class", "Extra.run": "method", // one class over both #if branches
		"Service": "interface", "Service.call": "method", "Service.stop": "method",
		"Color": "enum", "Level": "enum", "Level.isHigh": "method",
		"Meters": "abstract", "Meters.new": "method", "Meters.add": "method",
		"Options": "typedef", "Base": "class", "Base.run": "method",
		"helperAtModuleLevel": "func", "VERSION": "var",
	})
	langtest.CheckSymbols(t, results["src/shop/model/Cart.hx"], map[string]string{
		"Cart": "class", "Cart.new": "method", "Cart.total": "method", "CartItem": "class", "Items": "typedef",
	})
	langtest.CheckSymbols(t, results["src/shop/macros/Build.hx"], map[string]string{"Build": "class", "Build.run": "method"})
	langtest.CheckSymbols(t, results["build.hxml"], map[string]string{})
}

// Verifies: REQ-HAXE-002, REQ-HAXE-010
func TestLiteralsHideCode(t *testing.T) {
	source := `package a.b;
// import fake.Line;
/* import fake.Block;
   import fake.Block2; */
import real.One;
class A {
	var s = "import fake.Double; \" import fake.Escaped;";
	var t = 'import fake.Single ${ f("}") + '${ "nested" }' } $x import fake.After';
	var r = ~/import fake\/Regex;/gi;
	var u = 'it''s';
	function f() {
		#if (target.name == "js" && !debug)
		var x = real.pkg.Two.x;
		#elseif !neko
		var y = 1;
		#end
	}
}
import real.Three;
`
	extraction := extractSource([]byte(source))
	var got []string
	for _, rawImport := range extraction.Imports {
		got = append(got, rawImport.Name+" "+rawImport.Module)
	}
	want := []string{"import real.One", "import real.Three", "ref real.pkg.Two", "import.hx "}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("imports %q, want %q", got, want)
	}
	if packageName := readPackage([]byte("/* license */\n// x\npackage   a.b.c ;\nimport x.Y;")); packageName != "a.b.c" {
		t.Errorf("package %q", packageName)
	}
	if packageName := readPackage([]byte("import x.Y;\npackage a;")); packageName != "" {
		t.Errorf("package after an import: %q", packageName)
	}
}

// Verifies: REQ-HAXE-001
func TestClaims(t *testing.T) {
	var claimed []string
	classes := map[string]string{}
	for _, f := range langtest.Files(t, "testdata/repo") {
		if (Plugin{}).Claims(f) {
			claimed = append(claimed, f.Path)
			classes[f.Path] = Plugin{}.Class(f)
		}
	}
	sort.Strings(claimed)
	want := []string{"app/Project.xml", "app/extra/Util.hx", "app/source/app/Main.hx", "build.hxml", "common.hxml",
		"game/build.hxml", "game/haxe_libraries/acmeapi.hxml", "game/haxe_libraries/mylib.hxml",
		"game/haxe_libraries/tink_core.hxml", "game/haxe_libraries/tink_testrunner.hxml",
		"game/haxe_libraries/tink_unittest.hxml", "game/libs/mylib/src/mylib/Thing.hx", "game/src/Game.hx",
		"haxelib.json", "shared/Config.hx", "src/import.hx", "src/shop/Main.hx", "src/shop/macros/Build.hx",
		"src/shop/model/Cart.hx", "src/shop/util/Money.hx", "src/shop/util/Strings.hx", "test/TestAll.hx"}
	if !reflect.DeepEqual(claimed, want) {
		t.Errorf("claimed %q\nwant %q", claimed, want)
	}
	if classes["game/haxe_libraries/mylib.hxml"] != "lix:mylib.hxml" || classes["build.hxml"] != "hxml" {
		t.Errorf("classes %v", classes)
	}
	for _, source := range []string{
		`<?xml version="1.0"?><project name="x"><target name="build"/></project>`, // Ant
		`<project xmlns="http://maven.apache.org/POM/4.0.0"><modelVersion>4.0.0</modelVersion></project>`,
		`<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><Compile Include="a.fs"/></ItemGroup></Project>`,
		`<!-- <project><haxelib name="x"/></project> --><component/>`,
	} {
		if limeProject([]byte(source)) {
			t.Errorf("%s: read as a Lime project", source)
		}
	}
	if !limeProject([]byte("<?xml version=\"1.0\"?>\n<!-- c -->\n<project>\n<haxelib name=\"openfl\"/></project>")) {
		t.Error("an OpenFL project not read")
	}
}

// Verifies: REQ-HAXE-005
func TestHXML(t *testing.T) {
	h := readHXML([]byte("# comment\n-cp\nsrc\n--cwd sub\n-cp lib\n-L  a:1.0.0\n-lib ${X}\n-D a=b\n--next\nMain\n-js ::OUT::/x.js\n-cp ::TEMPLATE::\n"))
	if !reflect.DeepEqual(h.classPaths, []string{"src", "sub/lib"}) {
		t.Errorf("class paths %q", h.classPaths)
	}
	if len(h.libraries) != 1 || h.libraries[0] != (hxmlLibrary{"a", "1.0.0", 6}) {
		t.Errorf("libs %+v", h.libraries)
	}
	if h.defines["a"] != "b" {
		t.Errorf("defines %v", h.defines)
	}
	var specs []string
	for _, rawImport := range h.rawImports {
		specs = append(specs, rawImport.Spec)
	}
	if want := []string{"-cp src", "-cp lib", "-L a:1.0.0", "Main"}; !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %q, want %q", specs, want)
	}
}

// installHaxelib lays out a global haxelib repository: format 3.7.0 (current)
// and 3.5.0, a library in development (.dev) and one named in capitals.
func installHaxelib(t *testing.T) (repository, dev string) {
	repository, dev = t.TempDir(), t.TempDir()
	write := func(root, p, content string) {
		absolute := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(absolute), 0o755)
		os.WriteFile(absolute, []byte(content), 0o644)
	}
	write(repository, "format/.current", "3.7.0\n")
	write(repository, "format/3,7,0/haxelib.json", `{"name": "format", "classPath": "", "dependencies": {"hxcpp": "", "tink_core": "2.1.1"}}`)
	write(repository, "format/3,7,0/format/zip/Reader.hx", "package format.zip;\nclass Reader {}\n")
	write(repository, "format/3,5,0/format/old/Gone.hx", "package format.old;\nclass Gone {}\n")
	write(repository, "HxWidgets/.current", "1.0.0")
	write(repository, "HxWidgets/1,0,0/src/wx/Frame.hx", "package wx;\nclass Frame {}\n")
	write(repository, "HxWidgets/1,0,0/haxelib.json", `{"name": "hxWidgets", "classPath": "src"}`)
	write(repository, "devlib/.dev", dev)
	write(dev, "haxelib.json", `{"name": "devlib", "classPath": "src"}`)
	write(dev, "src/dev/Tool.hx", "package dev;\nclass Tool {}\n")
	return repository, dev
}

// Verifies: REQ-HAXE-008
func TestInstalledLibraries(t *testing.T) {
	root := t.TempDir()
	write := func(p, content string) {
		absolute := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(absolute), 0o755)
		os.WriteFile(absolute, []byte(content), 0o644)
	}
	write("build.hxml", "-cp src\n-lib format\n-lib hxwidgets\n-lib devlib\n")
	write("src/Main.hx", "import format.zip.Reader;\n")
	write("lix/haxe_libraries/coolib.hxml", "# @install: lix --silent download \"haxelib:/coolib#0.2.0\" into coolib/0.2.0/haxelib\n-lib tink_core\n-cp ${HAXE_LIBCACHE}/coolib/0.2.0/haxelib/src\n")
	write("lix/haxe_libraries/tink_core.hxml", "# @install: lix --silent download \"haxelib:/tink_core#2.1.1\" into tink_core/2.1.1/haxelib\n")
	write("lix/src/App.hx", "import cool.Thing;\n")
	repository, _ := installHaxelib(t)
	cache := t.TempDir()
	os.MkdirAll(filepath.Join(cache, "coolib/0.2.0/haxelib/src/cool"), 0o755)
	os.WriteFile(filepath.Join(cache, "coolib/0.2.0/haxelib/src/cool/Thing.hx"), []byte("package cool;\nclass Thing {}\n"), 0o644)
	environment := map[string]string{"HAXELIB_PATH": repository, "HAXE_LIBCACHE": cache}
	r := newResolver(root, langtest.Files(t, root), func(k string) string { return environment[k] })
	resolveImport := func(file, m string) lang.Target { return r.Resolve(file, lang.RawImport{Module: m, Name: kindImport}) }
	fmt37 := lang.Target{Ecosystem: ecosystemHaxelib, Package: "format", Version: "3.7.0", Floating: true}
	for m, want := range map[string]lang.Target{
		"format.zip.Reader": fmt37,
		"format.old.Gone":   fmt37, // not in the version in use; the table's name
		"wx.Frame":          {Ecosystem: ecosystemHaxelib, Package: "hxwidgets", Version: "1.0.0", Floating: true},
		"dev.Tool":          {Ecosystem: ecosystemHaxelib, Package: "devlib", Version: "dev", Floating: true},
	} {
		if got := resolveImport("src/Main.hx", m); got != want {
			t.Errorf("%s: %+v, want %+v", m, got, want)
		}
	}
	coolib := lang.Target{Ecosystem: ecosystemHaxelib, Package: "coolib", Version: "0.2.0", Pinned: true}
	if got := resolveImport("lix/src/App.hx", "cool.Thing"); got != coolib {
		t.Errorf("lix cache: %+v", got)
	}
	want := []lang.Target{
		{Ecosystem: ecosystemHaxelib, Package: "hxcpp", Floating: true},
		{Ecosystem: ecosystemHaxelib, Package: "tink_core", Version: "2.1.1", Pinned: true},
	}
	if got := r.Dependencies(fmt37); !reflect.DeepEqual(got, want) {
		t.Errorf("format dependencies %+v", got)
	}
	if got := r.Dependencies(coolib); !reflect.DeepEqual(got, []lang.Target{{Ecosystem: ecosystemHaxelib, Package: "tink_core", Version: "2.1.1", Pinned: true}}) {
		t.Errorf("coolib dependencies %+v", got)
	}
	if !r.Installed(fmt37) || r.Installed(coolib) {
		t.Error("format comes from haxelib, coolib from lix")
	}
	// Nothing installed: the table and the declared names decide.
	empty := t.TempDir()
	environment = map[string]string{"HAXELIB_PATH": empty, "HAXE_LIBCACHE": empty}
	r = newResolver(root, langtest.Files(t, root), func(k string) string { return environment[k] })
	if got := resolveImport("src/Main.hx", "wx.Frame"); got != (lang.Target{Ecosystem: ecosystemHaxelib, Package: "wx", Unresolved: true}) {
		t.Errorf("not installed: %+v", got)
	}
	if got := resolveImport("src/Main.hx", "format.zip.Reader"); got != (lang.Target{Ecosystem: ecosystemHaxelib, Package: "format", Floating: true}) {
		t.Errorf("declared, not installed: %+v", got)
	}
}

// Every prefix of every fixture file extracts, and long runs of the tokens
// that open or close something stay linear.
//
// Verifies: REQ-HAXE-010
func TestTruncated(t *testing.T) {
	var files []string
	filepath.Walk("testdata", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	for _, p := range files {
		source, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f := &scan.File{Path: strings.TrimPrefix(filepath.ToSlash(p), "testdata/repo/")}
		for i := 0; i <= len(source); i++ {
			if _, err := (Plugin{}).Extract(f, source[:i]); err != nil {
				t.Fatal(err)
			}
			extractSource(source[:i])
			readPackage(source[:i])
			readHXML(source[:i])
			readProject(source[:i])
			readHaxelib(source[:i])
		}
	}
	for _, unit := range []string{"{", "}", "(", ")", "[", "<", ">", "\"", "'", "'${", "${", "}'", "~/", "/*", "//x\n",
		"#if ", "#if (", "#else ", "#end ", "#elseif x ", "@:", "@:a(", "import ", "import a.", "a.", "a.B.", "using ",
		"class A ", "class A<", "enum abstract ", "function f() ", "function ", "var x ", "typedef T = ", "package ",
		"\\", "é", "<project>", "<haxelib name=\"", "<!--", "-cp ", "-lib x\n", "--macro a.B.c()\n", "=", ";"} {
		source := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		extractSource(source)
		readHXML(source)
		readProject(append([]byte("<project><haxelib/>"), source...))
		if d := time.Since(start); d > langtest.TimeLimit(5*time.Second) {
			t.Errorf("%q x %d: %v", unit, len(source)/len(unit), d)
		}
	}
}

// Verifies: REQ-HAXE-009
func TestIslands(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range (Plugin{}).Ecosystems() {
		ids[e.ID] = e.Std
	}
	if std, ok := ids[ecosystemStd]; !ok || !std || len(ids) != 2 || ids[ecosystemHaxelib] {
		t.Fatalf("ecosystems: %v", ids)
	}
	for f, r := range analyze(t) {
		for _, imported := range r.Imports {
			if e := imported.Target.Ecosystem; e != "" && e != ecosystemHaxelib && e != ecosystemStd {
				t.Errorf("%s: %s -> %s", f, imported.Spec, e)
			}
		}
	}
}

// An import.hx applies to the modules of its directory and below, up to the
// class path the module's package starts in: not the one above src/, and for a
// module whose directory does not spell its package, only its own directory's.
// A garbage import.hx is still the file that applies.
//
// Verifies: REQ-HAXE-004
func TestImportHx(t *testing.T) {
	t.Setenv("HAXELIB_PATH", t.TempDir())
	t.Setenv("HAXE_LIBCACHE", t.TempDir())
	root := langtest.Write(t, map[string]string{
		"import.hx":         "import haxe.Json;\n",
		"src/import.hx":     "using StringTools;\n",
		"src/a/import.hx":   "import a.b.C;\n",
		"src/a/b/import.hx": "import {{{ ;;; #if \"\n",
		"src/a/b/C.hx":      "package a.b;\nclass C {}\n",
		"odd/import.hx":     "import haxe.ds.StringMap;\n",
		"odd/D.hx":          "package x.y;\nclass D {}\n",
		"lone/E.hx":         "class E {}\n",
	})
	results := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, results["src/a/b/C.hx"], map[string]lang.Target{
		"import.hx (src/a/b/import.hx)": {Local: "src/a/b/import.hx"},
		"import.hx (src/a/import.hx)":   {Local: "src/a/import.hx"},
		"import.hx (src/import.hx)":     {Local: "src/import.hx"},
	})
	langtest.CheckImports(t, results["odd/D.hx"], map[string]lang.Target{
		"import.hx (odd/import.hx)": {Local: "odd/import.hx"},
	})
	langtest.CheckImports(t, results["lone/E.hx"], map[string]lang.Target{})
	langtest.CheckImports(t, results["src/a/import.hx"], map[string]lang.Target{
		"a.b.C": {Local: "src/a/b/C.hx"},
	})
}
