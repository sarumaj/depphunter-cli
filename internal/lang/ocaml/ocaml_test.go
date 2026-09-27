package ocaml

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// A dune project with two packages in dune-project (and their opam files, one
// with pin-depends, one lock): a wrapped library (an Import module, an interface,
// a Menhir grammar and an ocamllex lexer, a select, ppx rewriters), an executable
// that opens it by flags, a test, an unwrapped library under include_subdirs
// unqualified, a library under include_subdirs qualified, a toplevel script with
// #require, and a second project locked by dune package management. What dune
// builds (_build) and a local switch (_opam) are not read.
//
// Verifies: REQ-OCAML-001, REQ-OCAML-002, REQ-OCAML-003, REQ-OCAML-004, REQ-OCAML-005, REQ-OCAML-006
// Verifies: REQ-OCAML-007, REQ-OCAML-008, REQ-OCAML-009
func TestDuneProject(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	cmdliner := lang.Target{Ecosystem: ecoOpam, Package: "cmdliner", Version: "0123456789abcdef0123456789abcdef01234567", Origin: "https://github.com/dbuenzli/cmdliner.git", Pinned: true}
	re := lang.Target{Ecosystem: ecoOpam, Package: "re", Version: "main", Origin: "https://github.com/ocaml/ocaml-re.git", Floating: true}
	lwt := lang.Target{Ecosystem: ecoOpam, Package: "lwt", Version: "5.7.0", Requested: ">= 5.6 & < 6", Pinned: true}
	yojson := lang.Target{Ecosystem: ecoOpam, Package: "yojson", Version: "2.1.0", Pinned: true}
	fmt := lang.Target{Ecosystem: ecoOpam, Package: "fmt", Version: "0.9.0", Pinned: true}
	logs := lang.Target{Ecosystem: ecoOpam, Package: "logs", Version: "0.7.0", Pinned: true}
	ppxDeriving := lang.Target{Ecosystem: ecoOpam, Package: "ppx_deriving", Version: "5.2.1", Requested: ">= 5.0", Pinned: true}
	alcotest := lang.Target{Ecosystem: ecoOpam, Package: "alcotest", Floating: true}
	base := lang.Target{Ecosystem: ecoOpam, Package: "base", Unresolved: true}
	lockedLwt := lang.Target{Ecosystem: ecoOpam, Package: "lwt", Version: "5.9.1", Pinned: true}
	stdlib := lang.Target{Ecosystem: ecoStd, Package: "stdlib"}
	unix := lang.Target{Ecosystem: ecoStd, Package: "unix"}
	str := lang.Target{Ecosystem: ecoStd, Package: "str"}
	imports := map[string]map[string]lang.Target{
		"bin/dune": {
			"libraries: shop":     {Local: "lib/dune"},
			"libraries: cmdliner": cmdliner,
			"libraries: re":       re,
			"libraries: str":      str,
			"libraries: unix":     unix,
		},
		"bin/main.ml": {
			"open Cmdliner": cmdliner,
			"Cart":          {Local: "lib/cart.ml"},
			"Shop.Cart":     {Local: "lib/cart.ml"},
			"Shop.Price":    {Local: "lib/price.ml"},
			"Re":            re,
			"Str":           str,
			"Term":          cmdliner,
			"Cmd":           cmdliner,
		},
		"core/dune": {
			"libraries: base": base,
		},
		"core/shop_core.ml": {},
		"core/util/strings.ml": {
			"open Base": base,
			"String":    base,
		},
		"dune-project": {
			"depends: dune":         {Ecosystem: ecoOpam, Package: "dune", Floating: true},
			"depends: lwt":          lwt,
			"depends: yojson":       yojson,
			"depends: alcotest":     alcotest,
			"depends: ppx_deriving": ppxDeriving,
			"depends: fmt":          fmt,
			"depends: logs":         logs,
			"depends: conf-libev":   {Ecosystem: ecoOpam, Package: "conf-libev", Floating: true},
			"depends: shop":         {Local: "shop.opam"},
			"depends: cmdliner":     cmdliner,
			"depends: re":           re,
		},
		"dune-workspace": {},
		"lib/backend.default.ml": {
			"Sys": stdlib,
		},
		"lib/backend.unix.ml": {
			"Unix": unix,
		},
		"lib/cart.ml": {
			"open Import":     {Local: "lib/import.ml"},
			"Money":           {Local: "lib/import.ml"},
			"open Lwt.Syntax": lwt,
			"Lwt":             lwt,
			"Price":           {Local: "lib/price.ml"},
			"Json":            {Local: "lib/import.ml"},
			"List":            stdlib,
			"Strings":         {Local: "core/util/strings.ml"},
			"Logs":            logs,
			"Fmt":             fmt,
			"Lwt_unix":        lwt,
		},
		"lib/cart.mli": {
			"Price":       {Local: "lib/price.ml"},
			"Yojson.Safe": yojson,
		},
		"lib/dune": {
			"libraries: lwt":         lwt,
			"libraries: lwt.unix":    lwt,
			"libraries: yojson":      yojson,
			"libraries: fmt.tty":     fmt,
			"libraries: logs.fmt":    logs,
			"libraries: shop_core":   {Local: "core/dune"},
			"libraries: qual":        {Local: "qual/dune"},
			"libraries: unix":        unix,
			"pps: ppx_deriving.show": ppxDeriving,
		},
		"lib/import.ml": {
			"Yojson.Safe":       yojson,
			"Price":             {Local: "lib/price.ml"},
			"include Shop_core": {Local: "core/shop_core.ml"},
		},
		"lib/lexer.mll": {
			"open Parser": {Local: "lib/parser.mly"},
			"Lexing":      stdlib,
		},
		"lib/parser.mly": {
			"open Cart": {Local: "lib/cart.ml"},
			"Price":     {Local: "lib/price.ml"},
		},
		"lib/price.ml": {
			"Printf": stdlib,
		},
		"locked/dune-project": {
			"depends: lwt": lockedLwt,
		},
		"locked/src/app.ml": {
			"Lwt_main": lockedLwt,
			"Lwt":      lockedLwt,
		},
		"locked/src/dune": {
			"libraries: lwt": lockedLwt,
		},
		"qual/api.ml": {
			"Net.Client": {Local: "qual/net/client.ml"},
			"Net.Server": {Local: "qual/net/server.ml"},
		},
		"qual/dune": {},
		"qual/net/client.ml": {
			"Server": {Local: "qual/net/server.ml"},
		},
		"qual/net/server.ml": {},
		"scripts/setup.ml": {
			"#require \"yojson\"":   yojson,
			"#require \"lwt.unix\"": lwt,
			"Yojson.Safe":           yojson,
		},
		"shop-cli.opam": {
			"depends: dune":         {Ecosystem: ecoOpam, Package: "dune", Version: ">= 3.10", Floating: true},
			"depends: shop":         {Local: "shop.opam"},
			"depends: cmdliner":     cmdliner,
			"depends: re":           re,
			"pin-depends: cmdliner": cmdliner,
			"pin-depends: re":       re,
		},
		"shop.opam": {
			"depends: dune":         {Ecosystem: ecoOpam, Package: "dune", Version: ">= 3.10", Floating: true},
			"depends: lwt":          lwt,
			"depends: yojson":       yojson,
			"depends: alcotest":     alcotest,
			"depends: ppx_deriving": ppxDeriving,
			"depends: fmt":          fmt,
			"depends: logs":         logs,
			"depends: conf-libev":   {Ecosystem: ecoOpam, Package: "conf-libev", Floating: true},
			"depopts: graphics":     {Ecosystem: ecoOpam, Package: "graphics", Floating: true},
		},
		"shop.opam.locked": {
			"depends: lwt":           {Ecosystem: ecoOpam, Package: "lwt", Version: "5.7.0", Pinned: true},
			"depends: fmt":           fmt,
			"depends: logs":          logs,
			"depends: ppx_deriving":  {Ecosystem: ecoOpam, Package: "ppx_deriving", Version: "5.2.1", Pinned: true},
			"depends: ocplib-endian": {Ecosystem: ecoOpam, Package: "ocplib-endian", Version: "1.2", Pinned: true},
		},
		"test/dune": {
			"libraries: shop":     {Local: "lib/dune"},
			"libraries: alcotest": alcotest,
			"libraries: yojson":   yojson,
		},
		"test/test_shop.ml": {
			"Alcotest":    alcotest,
			"Shop.Price":  {Local: "lib/price.ml"},
			"Shop.Cart":   {Local: "lib/cart.ml"},
			"Yojson.Safe": yojson,
			"QCheck.Gen":  {Ecosystem: ecoOpam, Package: "qcheck-core", Unresolved: true},
		},
	}
	if len(res) != len(imports) {
		var got []string
		for f := range res {
			got = append(got, f)
		}
		t.Errorf("analyzed %d files, want %d: %v", len(res), len(imports), got)
	}
	for file, want := range imports {
		langtest.CheckImports(t, res[file], want)
	}
	symbols := map[string]map[string]string{
		"bin/dune":               {"executable main": "component"},
		"bin/main.ml":            {"cart": "value", "words": "value", "re": "value", "term": "value", "cmd": "value"},
		"core/dune":              {"library shop_core": "component"},
		"core/shop_core.ml":      {"version": "value"},
		"core/util/strings.ml":   {"trim": "function", "words": "function"},
		"dune-project":           {"package shop": "component", "package shop-cli": "component"},
		"dune-workspace":         {"context default": "component", "context release": "component"},
		"lib/backend.default.ml": {"now": "function"},
		"lib/backend.unix.ml":    {"now": "function"},
		"lib/cart.ml":            {"item": "type", "t": "type", "Empty": "exception", "empty": "value", "add": "function", "total": "function", "to_json": "function", "first": "function", "check": "function", "log": "function", "Id": "module", "Id.t": "type", "Id.counter": "value", "Id.next": "function", "Local": "module", "Local.x": "value", "uses_local": "value", "unix_time": "function", "s": "value", "c": "value", "q": "value", "counter": "class", "counter.incr": "method", "counter.get": "method", "raw_hash": "external"},
		"lib/cart.mli":           {"item": "type", "t": "type", "empty": "value", "add": "function", "total": "function", "to_json": "function", "Empty": "exception", "Id": "module", "Id.t": "type", "Id.next": "function"},
		"lib/dune":               {"library shop": "component"},
		"lib/import.ml":          {"Json": "module", "Money": "module"},
		"lib/lexer.mll":          {"letter": "value", "token": "rule", "comment": "rule"},
		"lib/parser.mly":         {"items": "rule", "item": "rule"},
		"lib/price.ml":           {"t": "type", "zero": "value", "add": "function", "+$": "function", "to_string": "function"},
		"locked/dune-project":    {"package locked": "component"},
		"locked/src/app.ml":      {},
		"locked/src/dune":        {"executable app": "component"},
		"qual/api.ml":            {"start": "function", "dir": "value"},
		"qual/dune":              {"library qual": "component"},
		"qual/net/client.ml":     {"connect": "function"},
		"qual/net/server.ml":     {"listen": "function"},
		"scripts/setup.ml":       {"json": "value"},
		"shop-cli.opam":          {},
		"shop.opam":              {},
		"shop.opam.locked":       {},
		"test/dune":              {"test test_shop": "component"},
		"test/test_shop.ml":      {"test_total": "function"},
	}
	for file, want := range symbols {
		langtest.CheckSymbols(t, res[file], want)
	}
	for _, s := range res["lib/cart.ml"].Symbols {
		if s.Name == "Id.next" && s.Line != 34 {
			t.Errorf("Id.next at line %d, want 34", s.Line)
		}
	}
}

// dune package management's lock directory answers --resolve-depth: a locked
// package's dependencies at their locked versions, without the compiler.
//
// Verifies: REQ-OCAML-009
func TestDuneLockDependencies(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	for _, tt := range []struct {
		pkg  lang.Target
		want []lang.Target
	}{
		{lang.Target{Ecosystem: ecoOpam, Package: "lwt", Version: "5.9.1"}, []lang.Target{
			{Ecosystem: ecoOpam, Package: "cppo", Version: "1.8.0", Pinned: true},
			{Ecosystem: ecoOpam, Package: "dune", Version: "3.17.2", Pinned: true},
			{Ecosystem: ecoOpam, Package: "ocplib-endian", Version: "1.2", Pinned: true},
		}},
		{lang.Target{Ecosystem: ecoOpam, Package: "ocplib-endian", Version: "1.2"}, []lang.Target{
			{Ecosystem: ecoOpam, Package: "cppo", Version: "1.8.0", Pinned: true},
			{Ecosystem: ecoOpam, Package: "dune", Version: "3.17.2", Pinned: true},
		}},
		// Another version than the lock's, and opam's own flat locks, answer nothing.
		{lang.Target{Ecosystem: ecoOpam, Package: "lwt", Version: "5.7.0"}, nil},
		{lang.Target{Ecosystem: ecoOpam, Package: "fmt", Version: "0.9.0"}, nil},
		{lang.Target{Ecosystem: ecoStd, Package: "unix"}, nil},
	} {
		if got := r.Dependencies(tt.pkg); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s %s: got %+v, want %+v", tt.pkg.Package, tt.pkg.Version, got, tt.want)
		}
	}
}

// What only looks like a module path - in nested comments (with strings inside
// them), strings, quoted strings, character literals, polymorphic variants, cppo
// lines, Menhir comments - is not read; a constructor is not a module; a module the
// file binds itself is not a reference.
//
// Verifies: REQ-OCAML-002, REQ-OCAML-003, REQ-OCAML-010, REQ-OCAML-011
func TestScannerHidesNonCode(t *testing.T) {
	src := "(* outer (* In.nested *) \"*)\" In.comment *)\n" +
		"let s = \"In.string \\\" still\" and c = '\"' and d = '\\'' \n" +
		"let q = {|In.quoted|} ^ {sql|In.Sql |sql} ^ {%ext|In.Ext|}\n" +
		"#if FOO\n" +
		"let from_if = Real.One.x\n" +
		"#else\n" +
		"let from_else = Real.Two.x\n" +
		"#endif\n" +
		"let v = `Poly.x\n" +
		"let k = Some (Ok None)\n" +
		"let f' x = x\n" +
		"type 'a t = 'a list\n" +
		"module Mine = struct let y = 1 end\n" +
		"let m = Mine.y\n" +
		"module F (Arg : Real.Sig) = struct include Arg end\n" +
		"module G = Real.Make (Real.Arg)\n" +
		"let p = (module Real.Packed : Real.S)\n" +
		"let r = r.Real.Field.x\n" +
		"let%expect_test \"t\" = Real.Test.run ()\n" +
		"let u = Real__Wrapped.x\n"
	ex, err := Plugin{}.Extract(&scan.File{Path: "m.ml"}, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var specs []string
	for _, im := range ex.Imports {
		specs = append(specs, im.Spec)
	}
	// Real.Sig and Real.S are module types: the module is Real.
	want := []string{"Real.One", "Real.Two", "Real", "Real.Make", "Real.Arg", "Real.Packed", "Real.Field", "Real.Test", "Real.Wrapped"}
	if !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %q, want %q", specs, want)
	}
	var names []string
	for _, s := range ex.Symbols {
		names = append(names, s.Name+" "+s.Kind)
	}
	wantSymbols := []string{"s value", "c value", "d value", "q value", "from_if value", "from_else value", "v value", "k value",
		"f' function", "t type", "Mine module", "Mine.y value", "m value", "F module", "G module", "p value", "r value", "u value"}
	if !reflect.DeepEqual(names, wantSymbols) {
		t.Errorf("symbols %q, want %q", names, wantSymbols)
	}
	grammar := "%{ open Ast %}\n/* it's Not.Code */\n%token <Tok.t> A\n%%\nmain: | A { Ast.x } // don't Read.This\n"
	ex, _ = Plugin{}.Extract(&scan.File{Path: "p.mly"}, []byte(grammar))
	specs = nil
	for _, im := range ex.Imports {
		specs = append(specs, im.Spec)
	}
	if want := []string{"open Ast", "Tok"}; !reflect.DeepEqual(specs, want) {
		t.Errorf("grammar imports %q, want %q", specs, want)
	}
}

// Build output and local switches are not the project's; dune files, opam files
// and locks, which share an empty or unusual extension, get cache classes of their
// own.
//
// Verifies: REQ-OCAML-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"src/a.ml": true, "src/a.mli": true, "src/lexer.mll": true, "src/parser.mly": true,
		"src/dune": true, "dune-project": true, "dune-workspace": true, "dune-workspace.dev": true,
		"shop.opam": true, "opam": true, "shop.opam.locked": true, "opam/opam": true,
		"shop.opam.template": false, "dune.lock/lwt.pkg": false, "README.md": false, "src/a.re": false,
		"_build/default/a.ml": false, "_opam/lib/lwt/lwt.mli": false, "_esy/x/a.ml": false,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("%s: claimed %v, want %v", p, got, want)
		}
	}
	classes := map[string]string{}
	for _, p := range []string{"dune", "dune-project", "dune-workspace", "opam", "x.opam.locked", "a.ml"} {
		c := lang.ClassOf(Plugin{}, &scan.File{Path: p})
		if other, ok := classes[c]; ok {
			t.Errorf("%s and %s share cache class %q", p, other, c)
		}
		classes[c] = p
	}
}

// dune's S-expressions: comments of all three kinds, strings with escapes and
// continuations, unclosed lists.
//
// Verifies: REQ-OCAML-006
func TestSexps(t *testing.T) {
	src := "; line\n#| block (library) |#\n(library #;(name hidden) (name \"a\\\"b\") (libraries x\n  y))\n(executable (name"
	got := parseSexps([]byte(src))
	if len(got) != 2 || got[0].value("name") != "a\"b" || len(got[0].field("libraries").atoms()) != 2 ||
		got[0].field("libraries").atoms()[1].line != 4 || got[1].head() != "executable" {
		t.Errorf("parsed %+v", got)
	}
}

// Every prefix of every fixture file, and inputs cut inside each construct, read
// without a panic; 100000 nested brackets do not exhaust the stack.
//
// Verifies: REQ-OCAML-011
func TestTruncated(t *testing.T) {
	var sources [][]byte
	filepath.Walk("testdata/repo", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			b, _ := os.ReadFile(p)
			sources = append(sources, b)
		}
		return nil
	})
	for _, s := range []string{"(*", "(* \"", "\"\\", "{|", "{id|", "{%ext ", "'\\", "'", "`", "#if", "#require \"",
		"module", "module type", "let (", "type (", "class [", "val (", "[%%x", "[@", "{", "Foo.", "open!"} {
		sources = append(sources, []byte("open A\n"+s))
	}
	for _, src := range sources {
		for i := 0; i <= len(src); i++ {
			for _, ext := range []string{".ml", ".mll", ".mly"} {
				readSource(src[:i], ext)
			}
			extractDune(src[:i])
			extractDuneProject(src[:i])
			extractWorkspace(src[:i])
			extractOpam(src[:i])
		}
	}
	deep := strings.Repeat("([{ struct begin sig object do ", 100000) + "open A"
	readSource([]byte(deep), ".ml")
	extractDune([]byte(strings.Repeat("(", 100000)))
	extractOpam([]byte("depends: " + strings.Repeat("[{(", 100000)))
}
