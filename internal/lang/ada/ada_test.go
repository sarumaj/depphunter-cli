package ada

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

// The fixture is an Alire crate "shop" (alire.toml: depends-on with ^, ~, =
// and * constraints and a case(os) alternative, pins to a git commit, a git
// branch on a private server and a directory) with a lock file in alire/
// (Alire 1.1 and later) and two crates Alire fetched into
// alire/cache/dependencies/ (aunit with its project file, and
// simple_components, whose unit Parsers does not spell its name). shop.gpr
// withs crates and a local project, sets Source_Dirs from an external's
// default, Main in both alternatives of a case construct and a Naming
// package. src/ holds a spec and body with a subunit, child units (a private
// one, a generic instantiation), a generic library unit, a unit whose file
// name GNAT would not guess, and hides fake with clauses in comments, strings
// and next to character literals and attribute ticks. tools/all_units.ada holds
// several units in one file; obj/ is what a build wrote beside the project
// file; libs/widgets is a pinned crate of the repository. The shared releases
// cache of Alire 2 (testdata/cache) holds two releases of utilada.

var (
	aunit     = lang.Target{Ecosystem: ecoAlire, Package: "aunit", Version: "24.0.0", Requested: "^24.0", Pinned: true}
	gnatcoll  = lang.Target{Ecosystem: ecoAlire, Package: "gnatcoll", Version: "25.0.0", Requested: "^25", Pinned: true}
	xmlada    = lang.Target{Ecosystem: ecoAlire, Package: "xmlada", Version: "24.0.0", Pinned: true}
	utilada   = lang.Target{Ecosystem: ecoAlire, Package: "utilada", Version: "2.6.0", Requested: "*", Floating: true}
	templates = lang.Target{Ecosystem: ecoAlire, Package: "templates_parser", Version: "24.0.0", Requested: "~24.0", Pinned: true}
	adaTOML   = lang.Target{Ecosystem: ecoAlire, Package: "ada_toml", Version: "669eacabc8cad45b89b0bfc34d99b4334d66bebd", Requested: "~0.5", Pinned: true}
	acmeLog   = lang.Target{Ecosystem: ecoAlire, Package: "acme_log", Version: "main", Floating: true, Origin: "https://git.acme.dev/acme_log.git"}
	win32ada  = lang.Target{Ecosystem: ecoAlire, Package: "win32ada", Version: "*", Floating: true}
	simple    = lang.Target{Ecosystem: ecoAlire, Package: "simple_components", Version: "4.62.0"}
)

func local(p string) lang.Target { return lang.Target{Local: p} }
func std(p string) lang.Target   { return lang.Target{Ecosystem: ecoStd, Package: p} }

// isolate points Alire's shared cache at testdata/cache, so what this machine
// has fetched cannot leak into the results.
func isolate(t *testing.T) {
	abs, err := filepath.Abs("testdata/cache")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALIRE_SETTINGS_DIR", "")
	t.Setenv("XDG_CACHE_HOME", abs)
	t.Setenv("HOME", t.TempDir())
}

func analyze(t *testing.T) map[string]*lang.FileResult {
	isolate(t)
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-ADA-002, REQ-ADA-004, REQ-ADA-006, REQ-ADA-007, REQ-ADA-008, REQ-ADA-011
func TestUnits(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["src/shop-main.adb"], map[string]lang.Target{
		"with Shop.Cart":               local("src/shop-cart.ads"),
		"with Ada.Text_IO":             std("ada.text_io"),
		"with AUnit.Assertions":        aunit,   // the crate Alire fetched that has it
		"with Parsers.Generic_Source":  simple,  // ditto, though the name does not spell it
		"with Util.Log.Loggers":        utilada, // the shared cache has its parent unit
		"with Templates_Parser":        templates,
		"with DOM.Core":                xmlada,  // the curated table
		"with TOML":                    adaTOML, // ditto, pinned by commit
		"with Acme_Log.Sinks":          acmeLog, // the name spells a pinned crate
		"with Widgets.Buttons":         local("libs/widgets/src/controls/widgets-buttons.ads"),
		"with Legacy_IO":               local("src/legacyio.ads"),
		"with Text_IO":                 std("text_io"),
		"with System.Storage_Elements": std("system.storage_elements"),
		"with Interfaces.C":            std("interfaces.c"),
		"with GNAT.OS_Lib":             std("gnat.os_lib"),
		"with Mystery.Thing":           {Ecosystem: ecoAlire, Package: "mystery", Unresolved: true},
		"with Shop_Config":             {}, // what Alire generates for the crate
		"with Tools":                   local("tools/all_units.ada"),
		"spec Shop.Main":               {}, // a subprogram body without a spec
		"parent Shop":                  local("src/shop.ads"),
	})
	langtest.CheckImports(t, res["src/shop-cart.ads"], map[string]lang.Target{
		"with Ada.Containers.Vectors": std("ada.containers"),
		"with Ada.Strings.Unbounded":  std("ada.strings"),
		"limited with Shop.Orders":    local("src/shop-orders.ads"),
		"private with Shop.Internal":  local("src/shop-internal.ads"),
		"with gnatcoll.json":          gnatcoll,
		"parent Shop":                 local("src/shop.ads"),
	})
	langtest.CheckImports(t, res["src/shop-cart.adb"], map[string]lang.Target{
		"with Ada.Text_IO": std("ada.text_io"),
		"with Shop.Log":    local("src/shop-log.ads"),
		"spec Shop.Cart":   local("src/shop-cart.ads"),
	})
	langtest.CheckImports(t, res["src/shop-cart-total.adb"], map[string]lang.Target{
		"separate (Shop.Cart)": local("src/shop-cart.adb"),
	})
	langtest.CheckImports(t, res["src/shop-orders.ads"], map[string]lang.Target{
		"limited private with Shop.Cart": local("src/shop-cart.ads"),
		"parent Shop":                    local("src/shop.ads"),
	})
	langtest.CheckImports(t, res["src/shop-log.ads"], map[string]lang.Target{
		"with Generic_Logger": local("src/generic_logger.ads"),
		"with Ada.Text_IO":    std("ada.text_io"),
		"parent Shop":         local("src/shop.ads"),
	})
	langtest.CheckImports(t, res["src/debug/debug_tool.adb"], map[string]lang.Target{
		"with Win32.Winbase": win32ada, // a case(os) alternative's crate
		"with Shop.Cart":     local("src/shop-cart.ads"),
		"spec Debug_Tool":    {},
	})
	langtest.CheckImports(t, res["libs/widgets/src/controls/widgets-buttons.ads"], map[string]lang.Target{
		"parent Widgets": local("libs/widgets/src/widgets.ads"),
	})
	langtest.CheckImports(t, res["tools/all_units.ada"], map[string]lang.Target{
		"spec Tools":      {}, // in the same file
		"with Tools":      {},
		"spec Tools_Main": {},
	})
}

// Verifies: REQ-ADA-005, REQ-ADA-006
func TestManifests(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["alire.toml"], map[string]lang.Target{
		"shop.gpr":         local("shop.gpr"),
		"aunit":            aunit,
		"gnatcoll":         gnatcoll,
		"xmlada":           xmlada,
		"utilada":          utilada,
		"templates_parser": templates,
		"ada_toml":         adaTOML,
		"widgets":          local("libs/widgets/alire.toml"),
		"win32ada":         win32ada,
		"acme_log":         acmeLog, // pinned without a dependency
	})
	langtest.CheckImports(t, res["shop.gpr"], map[string]lang.Target{
		`with "aunit"`:                    aunit, // the project file the fetched crate ships
		`with "gnatcoll"`:                 gnatcoll,
		`with "xmlada"`:                   xmlada,
		`with "libs/widgets/widgets.gpr"`: local("libs/widgets/widgets.gpr"),
		`with "config/shop_config.gpr"`:   {}, // Alire generates it
		`Source_Dirs "src"`:               local("src"),
		`Source_Dirs "src/debug"`:         local("src/debug"),
		`Source_Dirs "tools"`:             local("tools"),
		`Main "shop-main.adb"`:            local("src/shop-main.adb"),
		`Main "debug_tool"`:               local("src/debug/debug_tool.adb"),
	})
	langtest.CheckImports(t, res["libs/widgets/widgets.gpr"], map[string]lang.Target{
		`Source_Dirs "src/**"`: local("libs/widgets/src"),
	})
	if s := langtest.Symbols(t, res["alire.toml"]); !reflect.DeepEqual(s, map[string]string{"shop": "crate"}) {
		t.Errorf("alire.toml symbols %v", s)
	}
	if s := langtest.Symbols(t, res["shop.gpr"]); !reflect.DeepEqual(s, map[string]string{"Shop": "project"}) {
		t.Errorf("shop.gpr symbols %v", s)
	}
}

// Verifies: REQ-ADA-006
func TestPinRule(t *testing.T) {
	for c, want := range map[string]bool{
		"1.2.3": true, "=1.2.3": true, "= 1.2": true, "1.0.0-rc1": true,
		"^1.2": false, "~0.5": false, ">=1.0 & <2.0": false, "*": false, "/=1.0": false, "1.2.x": false,
		"669eacabc8cad45b89b0bfc34d99b4334d66bebd": false, "": false,
	} {
		if _, ok := exactVersion(c); ok != want {
			t.Errorf("exactVersion(%q) = %v", c, ok)
		}
	}
	root := t.TempDir()
	write(t, root, map[string]string{
		"alire.toml": "name = \"app\"\n[[depends-on]]\nlib_a = \"1.2.3\"\nlib_b = \"^1.2\"\nlib_c = \"*\"\nlib_d = \"^2\"\n" +
			"[[pins]]\nlib_d = { url = \"https://github.com/o/lib_d\" }\nlib_e = { version = \"=0.3.0\" }\n" +
			"lib_f = { path = \"/opt/lib_f\" }\n",
		"alire.lock": "[solution]\n[[solution.state]]\ncrate = \"lib_c\"\nversions = \"*\"\n[solution.state.release]\nversion = \"4.1.0\"\n",
	})
	isolate(t)
	r := newResolver(root, scanFiles(t, root))
	c := r.crates["."]
	for name, want := range map[string]lang.Target{
		"lib_a": {Ecosystem: ecoAlire, Package: "lib_a", Version: "1.2.3", Pinned: true},
		"lib_b": {Ecosystem: ecoAlire, Package: "lib_b", Version: "^1.2", Floating: true},
		// a lock file beside the manifest, as Alire wrote it before 1.1
		"lib_c": {Ecosystem: ecoAlire, Package: "lib_c", Version: "4.1.0", Requested: "*", Pinned: true},
		// a git pin without a commit follows the default branch
		"lib_d": {Ecosystem: ecoAlire, Package: "lib_d", Floating: true},
		"lib_e": {Ecosystem: ecoAlire, Package: "lib_e", Version: "0.3.0", Pinned: true},
		// a directory outside the repository
		"lib_f": {Ecosystem: ecoAlire, Package: "lib_f", Floating: true, Origin: "/opt/lib_f"},
	} {
		if got := r.crate(c, name); got != want {
			t.Errorf("%s: got %+v, want %+v", name, got, want)
		}
	}
}

// Verifies: REQ-ADA-003
func TestSymbols(t *testing.T) {
	res := analyze(t)
	for file, want := range map[string]map[string]string{
		"src/shop-cart.ads": {
			"Shop.Cart": "package", "Shop.Cart.Item": "struct", "Shop.Cart.Cart": "class",
			"Shop.Cart.Shape": "interface", "Shop.Cart.Count": "type", "Shop.Cart.Color": "enum",
			"Shop.Cart.Add": "procedure", "Shop.Cart.Total": "function", `Shop.Cart."+"`: "function",
			"Shop.Cart.Item_Vectors": "package", "Shop.Cart.Worker": "task", "Shop.Cart.Lock": "protected",
			"Shop.Cart.Lock.Seize": "procedure",
		},
		"src/shop-cart.adb": {
			// Trace is local to Add's body; a protected body's subprograms count
			"Shop.Cart.Add": "procedure", "Shop.Cart.Total": "function", `Shop.Cart."+"`: "function",
			"Shop.Cart.Lock.Seize": "procedure",
		},
		"src/shop-cart-total.adb": {"Shop.Cart.Total": "function"},
		"src/generic_logger.ads":  {"Generic_Logger": "package", "Generic_Logger.Info": "procedure"},
		"src/shop-log.ads":        {"Shop.Log": "package"},
		"src/shop.ads":            {"Shop": "package", "Shop.Money": "type"},
		"src/shop-orders.ads":     {"Shop.Orders": "package", "Shop.Orders.Order": "type"},
		"tools/all_units.ada":     {"Tools": "package", "Tools.Run": "procedure", "Tools_Main": "procedure"},
		"libs/widgets/src/controls/widgets-buttons.ads": {
			"Widgets.Buttons": "package", "Widgets.Buttons.Button": "class", "Widgets.Buttons.Click": "procedure",
		},
	} {
		if got := langtest.Symbols(t, res[file]); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v\nwant %v", file, got, want)
		}
	}
}

// Comments, strings (with doubled quotes), character literals (”' and '"')
// and attribute ticks (X'First, T'('x')) neither hide code nor make it up.
//
// Verifies: REQ-ADA-002, REQ-ADA-010, REQ-ADA-011
func TestLiteralsHideCode(t *testing.T) {
	src := []byte(`-- with Fake.A;
with Real.A; -- with Fake.B;
with Real.B, "Fake.C";
package P is
   S1 : constant String := "with ""Fake.D""; -- not a comment";
   C1 : constant Character := ''';
   C2 : constant Character := '"';
   N  : constant := 16#FF#E+2 + 1.0E-3 + 2#1010#;
   F  : constant Integer := Integer'First;
   Q  : constant Character := Character'('"');
   A  : constant Integer := Arr (1)'Length + X.all'Size;
   procedure After;
end P;
#if DEBUG then
with Real.C;
#else
with Fake.E;
#end if;
package Q is
   procedure Q1;
end Q;
`)
	s := extractSource(src)
	var specs []string
	for _, im := range s.imports {
		specs = append(specs, im.Spec)
	}
	want := []string{"with Real.A", "with Real.B", "with Real.C"}
	if !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %q, want %q", specs, want)
	}
	var names []string
	for _, sy := range s.symbols.List() {
		names = append(names, sy.Name)
	}
	if want := []string{"P", "P.After", "Q", "Q.Q1"}; !reflect.DeepEqual(names, want) {
		t.Errorf("symbols %q, want %q", names, want)
	}
	lx := newLexer([]byte("X'First = 'a' ''' \"a\"\"b\" T'('c') 16#1F# 1.5E+3"))
	var tokens []string
	for tk, ok := lx.next(); ok; tk, ok = lx.next() {
		tokens = append(tokens, string(tk.kind)+tk.text)
	}
	if want := []string{"iX", "p'", "iFirst", "p=", "c'a'", "c'''", `sa"b`, "iT", "p'", "p(", "c'c'", "p)", "n16#1F#", "n1.5E+3"}; !reflect.DeepEqual(tokens, want) {
		t.Errorf("tokens %q, want %q", tokens, want)
	}
}

// Verifies: REQ-ADA-001
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
	want := []string{"alire.toml", "libs/widgets/alire.toml", "libs/widgets/src/controls/widgets-buttons.ads",
		"libs/widgets/src/widgets.ads", "libs/widgets/widgets.gpr", "shop.gpr", "src/debug/debug_tool.adb",
		"src/generic_logger.ads", "src/legacyio.ads", "src/shop-cart-total.adb", "src/shop-cart.adb",
		"src/shop-cart.ads", "src/shop-internal.ads", "src/shop-log.ads", "src/shop-main.adb",
		"src/shop-orders.ads", "src/shop.ads", "tools/all_units.ada"}
	if !reflect.DeepEqual(claimed, want) {
		t.Errorf("claimed %q\nwant %q", claimed, want)
	}
	if classes["alire.toml"] != classManifest || classes["shop.gpr"] != "" {
		t.Errorf("classes %v", classes)
	}
	for _, f := range []*scan.File{{Path: "a.ads", Binary: true}, {Path: "pyproject.toml"}, {Path: "a.adx"}} {
		if (Plugin{}).Claims(f) {
			t.Errorf("%s claimed", f.Path)
		}
	}
	// alire/ and obj/ elsewhere are sources
	root := t.TempDir()
	write(t, root, map[string]string{"docs/alire/intro.ads": "package Intro is end Intro;\n", "obj/x.adb": "procedure X is begin null; end X;\n"})
	for _, f := range scanFiles(t, root) {
		if !(Plugin{}).Claims(f) {
			t.Errorf("%s not claimed", f.Path)
		}
	}
}

// Verifies: REQ-ADA-005
func TestProjectFiles(t *testing.T) {
	g := readGPR([]byte(`with "a", "b.gpr";
limited with "c";
aggregate library project Agg extends all "base.gpr" is
   Root := "src";
   Modes := ("x", "y");
   Other := Base'Source_Dirs;
   for Project_Files use ("p/one.gpr", "two.gpr");
   case Mode is
      when "a" | "b" => for Source_Dirs use (Root & "/a", Root);
      when others => for Source_Dirs use (Root & "/b") & Modes;
   end case;
   for Main use Other;
   package Naming is
      for Spec ("Pkg.Child") use "pkg__child.1.ada" at 1;
      for Implementation ("Pkg.Child") use "pkg__child.2.ada";
      for Dot_Replacement use "__";
   end Naming;
   package Builder renames Base.Builder;
end Agg;
`))
	items := func(list []item) []string {
		var out []string
		for _, i := range list {
			out = append(out, i.s)
		}
		return out
	}
	if g.name != "Agg" || g.extends == nil || g.extends.s != "base.gpr" {
		t.Errorf("header: %q extends %+v", g.name, g.extends)
	}
	if got := items(g.withs); !reflect.DeepEqual(got, []string{"a", "b.gpr", "c"}) {
		t.Errorf("withs %q", got)
	}
	if got := items(g.sourceDirs); !reflect.DeepEqual(got, []string{"src/a", "src", "src/b", "x", "y"}) {
		t.Errorf("source dirs %q", got)
	}
	if got := items(g.projectFiles); !reflect.DeepEqual(got, []string{"p/one.gpr", "two.gpr"}) {
		t.Errorf("project files %q", got)
	}
	if len(g.mains) != 0 {
		t.Errorf("a main from another project's attribute: %v", g.mains)
	}
	if g.specs["pkg.child"] != "pkg__child.1.ada" || g.bodies["pkg.child"] != "pkg__child.2.ada" {
		t.Errorf("naming %v %v", g.specs, g.bodies)
	}
	if d, rec := dirSpec(`src\**`); d != "src" || !rec {
		t.Errorf("dirSpec: %q %v", d, rec)
	}
}

// Verifies: REQ-ADA-006, REQ-ADA-008
func TestInstalledCrates(t *testing.T) {
	isolate(t)
	root, _ := filepath.Abs("testdata/repo")
	r := newResolver(root, langtest.Files(t, "testdata/repo"))
	for _, tc := range []struct {
		t         lang.Target
		deps      []lang.Target
		installed bool
	}{
		// the lock file's solution: libgpr at the version it chose
		{gnatcoll, []lang.Target{{Ecosystem: ecoAlire, Package: "libgpr", Version: "25.0.1", Requested: "^25", Pinned: true}}, false},
		{aunit, nil, false},
		// a crate Alire fetched that the lock does not list: its own manifest
		{simple, []lang.Target{
			{Ecosystem: ecoAlire, Package: "strings_edit", Version: "^4.0", Floating: true},
			{Ecosystem: ecoAlire, Package: "tables", Version: "1.14.0", Pinned: true},
		}, true},
		{utilada, nil, true},
		{lang.Target{Ecosystem: ecoStd, Package: "ada.text_io"}, nil, false},
	} {
		if got := r.Dependencies(tc.t); !reflect.DeepEqual(got, tc.deps) {
			t.Errorf("%s: got %+v, want %+v", tc.t.Package, got, tc.deps)
		}
		if got := r.Installed(tc.t); got != tc.installed {
			t.Errorf("%s: installed %v", tc.t.Package, got)
		}
	}
	// the shared cache is read only for crates the manifest names, at the
	// locked version, else the newest
	c := r.crates["."]
	if ic := c.installed["utilada"]; ic == nil || ic.version != "2.6.0" {
		t.Errorf("utilada from the shared cache: %+v", ic)
	}
	if _, ok := c.units["util.log"]; !ok {
		t.Errorf("units of the shared cache not indexed: %v", c.units)
	}
	// a lock file's link to a git pin (Alire keeps its checkout in alire/cache/pins)
	lock := readLock([]byte("[solution]\n[[solution.state]]\ncrate = \"lib\"\nfulfilment = \"linked\"\n" +
		"[solution.state.link]\nurl = \"git+https://git.acme.dev/lib.git\"\ncommit = \"0123456789abcdef0123456789abcdef01234567\"\npath = \"alire/cache/pins/lib\"\n"))
	c.lock["lib"] = lock["lib"]
	want := lang.Target{Ecosystem: ecoAlire, Package: "lib", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true, Origin: "https://git.acme.dev/lib.git"}
	if got := r.crate(c, "lib"); got != want {
		t.Errorf("linked git pin: %+v", got)
	}
}

// Verifies: REQ-ADA-009
func TestIslands(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range (Plugin{}).Ecosystems() {
		ids[e.ID] = e.Std
	}
	if std, ok := ids[ecoStd]; !ok || !std || len(ids) != 2 || ids[ecoAlire] {
		t.Fatalf("ecosystems: %v", ids)
	}
	for f, r := range analyze(t) {
		for _, im := range r.Imports {
			if e := im.Target.Ecosystem; e != "" && e != ecoAlire && e != ecoStd {
				t.Errorf("%s: %s -> %s", f, im.Spec, e)
			}
		}
	}
}

// Every prefix of every fixture file, and long runs of each construct, are
// read without a panic and in linear time. A truncated file cut the scanner
// off inside a string, a character literal, a gnatprep branch or a header.
//
// Verifies: REQ-ADA-010
func TestTruncated(t *testing.T) {
	var files []string
	filepath.Walk("testdata", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	for _, p := range files {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f := &scan.File{Path: filepath.Base(p)}
		for i := 0; i <= len(src); i++ {
			if _, err := (Plugin{}).Extract(f, src[:i]); err != nil {
				t.Fatal(err)
			}
			extractSource(src[:i])
			units(src[:i], true)
			readGPR(src[:i])
			readManifest(src[:i])
			readLock(src[:i])
		}
	}
	for _, unit := range []string{"(", ")", "'", "''", "'''", "\"", "\"\"", "--", "#if x then\n", "#else\n", "#end if;\n",
		"with ", "with A.", "limited ", "private ", "separate (", "package ", "package body P is ", "procedure P is ",
		"function F return T is ", "begin ", "end ", "end if; ", "if x then ", "loop ", "declare ", "case x is ",
		"type T is record ", "type T is ", "generic ", "task type T is ", "protected body P is ", "entry E when x is ",
		"accept E do ", "X'", "16#", "1.0E", "[", "for Source_Dirs use (", "project P is ", "case M is when \"a\" => ",
		"external (\"A\", ", "A & ", "package Naming is ", "é", ";", "is "} {
		src := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		extractSource(src)
		units(src, true)
		readGPR(src)
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%q x %d: %v", unit, len(src)/len(unit), d)
		}
	}
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func scanFiles(t *testing.T, root string) []*scan.File {
	t.Helper()
	return langtest.Files(t, root)
}
