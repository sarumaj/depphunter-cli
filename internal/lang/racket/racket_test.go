package racket

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

// The fixture is a package "shop" (info.rkt: collection "shop", deps with
// #:version minimums, a git commit, a git branch, a private git server with
// ?path= and a tag, a #:checksum, a local package and a directory outside the
// repository; build-deps; entries hidden in comments). main.rkt requires in
// every form (only-in, prefix-in, rename-in, except-in, for-syntax,
// for-label, submod of itself and of a file, file, lib, planet, the own
// collection, a local multi-collection package's collection, a collects/
// tree, declared, table and unknown collections, base modules) and hides
// fake requires in comments, datum comments, block comments, a here string,
// a regexp, a string and a quoted list; it defines in every definition form
// and has module, module+ and module* submodules. util.rkt is Typed Racket,
// sub/helpers.rkt reads at-exp, docs/ holds Scribble documents, bin/shop-cli
// is a racket script without an extension, tests/ has load files, a module
// form without #lang, DrRacket's WXME format and a teaching-language file
// with a #reader line. scheme/ has a .scm with #lang (claimed) and Chez and
// R6RS programs (not); compiled/ is raco make's output; libs/widgets-lib is
// a multi-collection package.

var (
	rackunit  = lang.Target{Ecosystem: ecoRaco, Package: "rackunit-lib", Version: "1.10", Floating: true}
	threading = lang.Target{Ecosystem: ecoRaco, Package: "threading", Floating: true}
	gregor    = lang.Target{Ecosystem: ecoRaco, Package: "gregor-lib", Version: "0.3", Floating: true}
	rebellion = lang.Target{Ecosystem: ecoRaco, Package: "rebellion", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true}
	guard     = lang.Target{Ecosystem: ecoRaco, Package: "guard", Version: "main", Floating: true}
	acmeLog   = lang.Target{Ecosystem: ecoRaco, Package: "acme-log-lib", Version: "v1.4.0", Origin: "https://git.acme.dev/racket/acme-log.git"}
	typed     = lang.Target{Ecosystem: ecoRaco, Package: "typed-racket", Version: "fedcba9876543210fedcba9876543210fedcba98", Pinned: true}
	scribble  = lang.Target{Ecosystem: ecoRaco, Package: "scribble-lib", Floating: true}
	gui       = lang.Target{Ecosystem: ecoRaco, Package: "gui-lib", Floating: true}
)

func local(p string) lang.Target { return lang.Target{Local: p} }
func std(p string) lang.Target   { return lang.Target{Ecosystem: ecoStd, Package: p} }
func unresolved(p string) lang.Target {
	return lang.Target{Ecosystem: ecoRaco, Package: p, Unresolved: true}
}

func analyze(t *testing.T) map[string]*lang.FileResult {
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-RACKET-002, REQ-RACKET-004, REQ-RACKET-005, REQ-RACKET-007, REQ-RACKET-008
func TestRequires(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["main.rkt"], map[string]lang.Target{
		"#lang racket/base":                       std("racket"),
		"require racket/list":                     std("racket"),
		`require "util.rkt"`:                      local("util.rkt"),
		`require "sub/helpers.rkt"`:               local("sub/helpers.rkt"),
		"require racket/string":                   std("racket"),
		"require threading":                       threading, // the table's threading-lib, declared as its umbrella
		"require rackunit":                        rackunit,
		"require racket/match":                    std("racket"),
		"require racket/base":                     std("racket"),
		"require syntax/parse":                    std("syntax"),
		"require racket/contract":                 std("racket"),
		`require (submod "." inner)`:              {},
		`require "sub/extra.rkt"`:                 local("sub/extra.rkt"), // (submod "sub/extra.rkt" tool)
		`require (file "/opt/racket/shared.rkt")`: {},
		`require (lib "gregor/main.rkt")`:         gregor,
		"require (planet jaymccarthy/sqlite:5:1)": {Ecosystem: ecoRaco, Package: "planet/jaymccarthy/sqlite", Version: "5.1", Floating: true},
		"require shop/util":                       local("util.rkt"),
		"require widgets/button":                  local("libs/widgets-lib/widgets/button.rkt"),
		"require rebellion/collection/list":       rebellion, // a git dep, named as raco names it
		"require acme-log/core":                   acmeLog,   // ?path=acme-log-lib, the collection without -lib
		"require guard":                           guard,
		"require typed/racket/unsafe":             typed, // typed-racket-lib's umbrella package
		"require net/url":                         std("net"),
		"require net/smtp":                        unresolved("net-lib"),
		"require plot/pict":                       unresolved("plot-lib"),
		"require mystery/thing":                   unresolved("mystery"),
		"require data/queue":                      std("data"),
		"require data/gvector":                    unresolved("data-lib"),
		"require compat":                          local("collects/compat/main.rkt"),
		"module racket/base":                      std("racket"),
		"require racket/function":                 std("racket"),
	})
	langtest.CheckImports(t, res["util.rkt"], map[string]lang.Target{
		"#lang typed/racket":              typed,
		`require/typed "sub/helpers.rkt"`: local("sub/helpers.rkt"),
		"require/opaque-type gregor":      gregor,
		"module racket/base":              std("racket"),
		"require racket/format":           std("racket"),
	})
	langtest.CheckImports(t, res["sub/helpers.rkt"], map[string]lang.Target{
		"#lang at-exp":                 unresolved("at-exp-lib"),
		"#lang racket/base":            std("racket"),
		"require racket/require":       std("racket"),
		"require racket/format":        std("racket"), // (multi-in racket (format string))
		"require racket/string":        std("racket"),
		`require (path-up "util.rkt")`: local("util.rkt"),
	})
	langtest.CheckImports(t, res["libs/widgets-lib/widgets/button.rkt"], map[string]lang.Target{
		"#lang racket/base":             std("racket"),
		"require racket/gui/base":       gui, // declared by widgets-lib, not by shop
		"require widgets/private/style": local("libs/widgets-lib/widgets/private/style.rkt"),
	})
	langtest.CheckImports(t, res["libs/widgets-lib/widgets/main.rkt"], map[string]lang.Target{
		"#lang racket/base":    std("racket"),
		`require "button.rkt"`: local("libs/widgets-lib/widgets/button.rkt"),
	})
	langtest.CheckImports(t, res["bin/shop-cli"], map[string]lang.Target{
		"#lang racket/base":      std("racket"),
		`require "../main.rkt"`:  local("main.rkt"),
		"require racket/cmdline": std("racket"),
	})
	langtest.CheckImports(t, res["scheme/legacy.scm"], map[string]lang.Target{
		"#lang racket":          std("racket"),
		`require "../util.rkt"`: local("util.rkt"),
	})
	langtest.CheckImports(t, res["tests/load-me.rktl"], map[string]lang.Target{
		`load "helpers.rktl"`: local("tests/helpers.rktl"),
		"require rackunit":    rackunit,
	})
	langtest.CheckImports(t, res["tests/old-style.rkt"], map[string]lang.Target{
		"module racket/base":  std("racket"),
		"require racket/list": std("racket"),
	})
	langtest.CheckImports(t, res["tests/beginner.rkt"], map[string]lang.Target{
		`#reader (lib "htdp-beginner-reader.ss" "lang")`: unresolved("htdp-lib"),
		"require 2htdp/image":                            unresolved("htdp-lib"),
	})
	langtest.CheckImports(t, res["tests/snip.rkt"], nil) // DrRacket's WXME format
	langtest.CheckImports(t, res["data/prices.rktd"], nil)
	langtest.CheckImports(t, res["collects/compat/main.rkt"], map[string]lang.Target{"#lang racket/base": std("racket")})
}

// Verifies: REQ-RACKET-002, REQ-RACKET-010, REQ-RACKET-011
func TestScribble(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["docs/manual.scrbl"], map[string]lang.Target{
		"#lang scribble/manual":         scribble, // a build-dep
		"require racket/base":           std("racket"),
		"require shop":                  local("main.rkt"), // the collection's main.rkt
		"require scribble/example":      scribble,
		`include-section "intro.scrbl"`: local("docs/intro.scrbl"),
	})
	langtest.CheckSymbols(t, res["docs/manual.scrbl"], map[string]string{"shop-eval": "var"})
	langtest.CheckImports(t, res["docs/intro.scrbl"], map[string]lang.Target{"#lang scribble/manual": scribble})
}

// Verifies: REQ-RACKET-003
func TestSymbols(t *testing.T) {
	res := analyze(t)
	langtest.CheckSymbols(t, res["main.rkt"], map[string]string{
		"greeting": "var", "pattern": "var", "open-paren": "var", "space": "var", "label": "var",
		"checkout": "func", "adder": "func", "low": "var", "high": "var", "swap!": "macro",
		"item": "struct", "sale": "struct", "cart%": "class", "cart%.total": "method", "cart%.log-line": "method",
		"prices": "var", "sizes": "var", "odd name": "var", "3d-view": "var", "show": "func",
		"inner": "module", "inner.x": "var", "test": "module", "main": "module", "main.entry": "var",
	})
	langtest.CheckSymbols(t, res["util.rkt"], map[string]string{"Price": "type", "total": "func", "helpers": "module", "helpers.fmt": "func"})
	langtest.CheckSymbols(t, res["sub/helpers.rkt"], map[string]string{"greet": "func", "shout": "var"})
	langtest.CheckSymbols(t, res["tests/old-style.rkt"], map[string]string{"legacy": "var"})
	langtest.CheckSymbols(t, res["data/prices.rktd"], map[string]string{})
	langtest.CheckSymbols(t, res["info.rkt"], map[string]string{})
}

// Verifies: REQ-RACKET-006
func TestInfo(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["info.rkt"], map[string]lang.Target{
		`deps "base"`:         std("base"),
		`deps "rackunit-lib"`: rackunit,
		`deps "threading"`:    threading,
		`deps "gregor-lib"`:   gregor,
		`deps "https://github.com/jackfirth/rebellion.git#0123456789abcdef0123456789abcdef01234567"`: rebellion,
		`deps "git://github.com/acme/guard#main"`:                                                    guard,
		`deps "https://git.acme.dev/racket/acme-log.git?path=acme-log-lib#v1.4.0"`:                   acmeLog,
		`deps "typed-racket"`:       typed,
		`deps "widgets-lib"`:        local("libs/widgets-lib/info.rkt"),
		`deps "../outside-pkg"`:     {Ecosystem: ecoRaco, Package: "outside-pkg", Floating: true, Origin: "../outside-pkg"},
		`build-deps "scribble-lib"`: scribble,
		`build-deps "racket-doc"`:   {Ecosystem: ecoRaco, Package: "racket-doc", Floating: true},
	})
	langtest.CheckImports(t, res["libs/widgets-lib/info.rkt"], map[string]lang.Target{
		`deps "base"`:    std("base"),
		`deps "gui-lib"`: gui,
	})
	// quasiquoted deps with an unquoted version, (list ...) calls, sources
	in := readInfo([]byte("#lang info\n(define collection 'multi)\n(define version \"9.3\")\n" +
		"(define deps `(\"racket-lib\" [\"racket\" #:version ,version] (list \"x-lib\" '#:version \"2.0\")))\n" +
		"(define build-deps (append '(\"a-doc\") (list \"github://github.com/o/r/dev/sub/pkg\")))\n"))
	if in.collection != "multi" || !in.pkg || in.version != "9.3" {
		t.Errorf("info: %+v", in)
	}
	var got []string
	for _, d := range in.deps {
		got = append(got, strings.Join([]string{d.name, d.version, d.url, d.ref, boolText(d.build)}, "|"))
	}
	want := []string{"racket-lib||||", "racket||||", "x-lib|2.0|||", "a-doc||||build", "pkg||https://github.com/o/r|dev|build"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("deps %q\nwant %q", got, want)
	}
}

func boolText(b bool) string {
	if b {
		return "build"
	}
	return ""
}

// Verifies: REQ-RACKET-010
func TestReader(t *testing.T) {
	m := Read([]byte("#!/usr/bin/env racket\n; c\n#| a |# #lang   typed/racket/base  #:with-refinements\n"+
		"#;(skip) #;#;(a) (b) [x {y}] #hash((1 . 2)) #s(p 1) #(v) #\\) #\\space #\\λ #:kw |a b| a\\ b 3arg 1.5e3 -1/2 "+
		"'q `(qq ,u ,@us) #'s #`qs #,us #,@uss #&box #<<END\n(fake)\nEND\n#rx\"(\" #px#\"[\" #\"bytes\" #0=(cyc . #0#) #ci Up\n"), false)
	if m.Lang != "typed/racket/base #:with-refinements" || m.LangLine != 3 {
		t.Errorf("lang %q line %d", m.Lang, m.LangLine)
	}
	var got []string
	for _, f := range m.Forms {
		got = append(got, show(f))
	}
	want := []string{"(x (y))", "hash:((1 . 2))", "s:(p 1)", "vector:(v)", `#\)`, `#\space`, `#\λ`, "#:kw", "a b", `a\ b`, "3arg", "1.5e3", "-1/2",
		"(quote q)", "(quasiquote (qq (unquote u) (unquote-splicing us)))", "(syntax s)", "(quasisyntax qs)", "(unsyntax us)",
		"(unsyntax-splicing uss)", "box", "(fake)\n", `#rx(`, `#rx[`, "bytes", "(cyc . #0#)", "Up"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("forms\n%q\nwant\n%q", got, want)
	}
	// Scribble: text is skipped, @-forms are read
	d := Read([]byte("#lang scribble/manual\n@title[#:style 'toc]{A @bold{b} (not) @; gone (x)\n}\n@(f 1) @g|{ @(h) }| @\"s\" @{t @k}"), false)
	got = nil
	for _, f := range d.Forms {
		got = append(got, show(f))
	}
	want = []string{"(title #:style (quote toc) (bold))", "(f 1)", "(g)", "s", "(k)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scribble forms %q\nwant %q", got, want)
	}
}

// show writes a datum back for TestReader.
func show(n *Node) string {
	switch n.Kind {
	case List:
		var parts []string
		for _, k := range n.Kids {
			parts = append(parts, show(k))
		}
		s := "(" + strings.Join(parts, " ") + ")"
		if n.Tag != "" {
			s = n.Tag + ":" + s
		}
		return s
	case Keyword:
		return "#:" + n.Text
	}
	return n.Text
}

// Verifies: REQ-RACKET-001
func TestClaims(t *testing.T) {
	var claimed []string
	classes := map[string]string{}
	for _, f := range langtest.Files(t, "testdata/repo") {
		if (Plugin{}).Claims(f) {
			claimed = append(claimed, f.Path)
			classes[f.Path] = (Plugin{}).Class(f)
		}
	}
	sort.Strings(claimed)
	want := []string{"bin/shop-cli", "collects/compat/main.rkt", "data/prices.rktd", "docs/intro.scrbl", "docs/manual.scrbl",
		"info.rkt", "libs/widgets-lib/info.rkt", "libs/widgets-lib/widgets/button.rkt", "libs/widgets-lib/widgets/main.rkt",
		"libs/widgets-lib/widgets/private/style.rkt", "main.rkt", "scheme/legacy.scm", "sub/extra.rkt", "sub/helpers.rkt",
		"tests/beginner.rkt", "tests/helpers.rktl", "tests/load-me.rktl", "tests/old-style.rkt", "tests/snip.rkt", "util.rkt"}
	if !reflect.DeepEqual(claimed, want) {
		t.Errorf("claimed %q\nwant %q", claimed, want)
	}
	if classes["info.rkt"] != classInfo || classes["main.rkt"] != "" {
		t.Errorf("classes %v", classes)
	}
	for _, f := range []*scan.File{{Path: "a.rkt", Binary: true}, {Path: "x.ss", Lang: "Scheme"}, {Path: "x.scm"},
		{Path: "compiled/a.rkt"}, {Path: "src/compiled/errortrace/a.rkt"}, {Path: "run"}} {
		if (Plugin{}).Claims(f) {
			t.Errorf("%s claimed", f.Path)
		}
	}
	for _, f := range []*scan.File{{Path: "x.ss", Lang: "Racket"}, {Path: "run", Lang: "Racket"}, {Path: "compiledx/a.rkt"}} {
		if !(Plugin{}).Claims(f) {
			t.Errorf("%s not claimed", f.Path)
		}
	}
}

// Verifies: REQ-RACKET-009
func TestIslands(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range (Plugin{}).Ecosystems() {
		ids[e.ID] = e.Std
	}
	if std, ok := ids[ecoStd]; !ok || !std || len(ids) != 2 || ids[ecoRaco] {
		t.Fatalf("ecosystems: %v", ids)
	}
	for f, r := range analyze(t) {
		for _, im := range r.Imports {
			if e := im.Target.Ecosystem; e != "" && e != ecoRaco && e != ecoStd {
				t.Errorf("%s: %s -> %s", f, im.Spec, e)
			}
		}
	}
}

// Every prefix of every fixture file, and long runs of each construct, are
// read without a panic and in linear time. Cut off, a file ends inside a
// string, a block comment, a here string, an @-form's text or a list.
//
// Verifies: REQ-RACKET-010
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
		for _, ext := range []string{".rkt", ".scrbl"} {
			f := &scan.File{Path: "x" + ext}
			for i := 0; i <= len(src); i++ {
				if _, err := (Plugin{}).Extract(f, src[:i]); err != nil {
					t.Fatal(err)
				}
			}
		}
		for i := 0; i <= len(src); i++ {
			extractInfo(src[:i])
		}
	}
	for _, unit := range []string{"(", ")", "[", "]", "{", "}", "\"", "#|", "|#", "#;", "#<<E\n", "#\\", "#\\a", "#:", "#hash(",
		"'", "`", ",", ",@", "#'", "#`", "#,@", "#&", "|", "\\", "@", "@x{", "@x[", "@x|{", "@;{", "@(", "}|", "#0=", "#rx\"",
		"(require ", "(require (only-in ", "(module m racket ", "(module+ t ", "(define (", "(struct ", "(define x (class o ",
		"(lib \"", "(submod ", "#reader", "#lang ", "é", ";", "#!", "#!/", "(define deps '(", "(\"a\" #:version "} {
		src := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		extractSource(src, ".rkt")
		extractSource(src, ".scrbl")
		extractSource(append([]byte("#lang at-exp racket\n"), src...), ".rkt")
		readInfo(src)
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%q x %d: %v", unit, len(src)/len(unit), d)
		}
	}
}
